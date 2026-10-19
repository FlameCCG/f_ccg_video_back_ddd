package mq

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	media "f_ccg_video_back_ddd/internal/domain/media"
)

// buildFilterComplex 构造多输出 filter_complex，并回填各目标的 map 标签。
// KeepSource 目标走 null 滤镜（绝不上采样）；GPU + 硬件缩放时用 scale_cuda。
func buildFilterComplex(targets []media.TranscodeTarget, useGPU, useHardwareScale bool) string {
	if len(targets) == 0 {
		return ""
	}

	var b strings.Builder

	fmt.Fprintf(&b, "[0:v]split=%d", len(targets))

	for i := range targets {
		fmt.Fprintf(&b, "[vraw%d]", i)
	}

	b.WriteString(";")

	for i := range targets {
		label := fmt.Sprintf("out%d", i)

		targets[i].MapLabel = label

		switch {
		case targets[i].KeepSource:
			fmt.Fprintf(&b, "[vraw%d]null[out%d];", i, i)
		case useGPU && useHardwareScale:
			// 硬件缩放：数据已经在显存中，直接 scale_cuda
			fmt.Fprintf(&b, "[vraw%d]scale_cuda=-2:%d[out%d];", i, targets[i].Resolution, i)
		default:
			// 软件缩放
			fmt.Fprintf(&b, "[vraw%d]scale=-2:%d[out%d];", i, targets[i].Resolution, i)
		}
	}

	return strings.TrimSuffix(b.String(), ";")
}

// buildFFmpegArgsForOutputs 构造多输出 MP4 编码参数。
//
// chunk 非 nil 时进入分段模式：-ss/-t 只编指定时间段、不 map 音频（共享音轨单独编）、
// 不加 faststart（中间产物，终段拼装时统一重封装）、可选按秒强制关键帧（DASH 网格）。
func buildFFmpegArgsForOutputs(
	policy *media.TranscodePolicy,
	srcPath, filter string,
	targets []media.TranscodeTarget,
	outputs []string,
	useGPU, useHardwareScale bool,
	sourceBitrate int,
	sourceCodec string,
	chunk *media.FFmpegChunkSpan,
) []string {
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}

	threads := policy.ThreadsOrZero()

	if useGPU {
		args = append(args, "-hwaccel", "cuda")

		if useHardwareScale {
			// 关键：多分辨率流处理时，为了使用 scale_cuda，必须让 decoder 输出保持在显存
			args = append(args, "-hwaccel_output_format", "cuda")
		}
	}

	// 分段模式：输入侧精确 seek 到块起点（关键帧 seek + 解码丢弃到帧级精确）。
	if chunk != nil && chunk.StartSec > 0 {
		args = append(args, "-ss", strconv.Itoa(chunk.StartSec))
	}

	args = append(args, "-i", srcPath, "-filter_complex", filter)

	// 全局应用线程配置
	if threads > 0 {
		args = append(args, "-threads", strconv.Itoa(threads))
	}

	// 注意：GPU 和 CPU 的 Preset 参数名虽然通用，但含义不同，不过都用 -preset 传入没问题。
	args = append(args, "-preset", policy.PresetOrDefault(useGPU))

	// 编码器
	videoCodec := "libx264"
	if useGPU {
		videoCodec = "h264_nvenc"
	}

	// 针对每一个输出流
	for idx, t := range targets {
		args = append(args, "-map", fmt.Sprintf("[%s]", t.MapLabel))

		if chunk == nil || !chunk.VideoOnly {
			args = append(args, "-map", "0:a?")
		}

		args = append(args, "-c:v:0", videoCodec)

		if chunk == nil || !chunk.VideoOnly {
			// 音频编码
			args = append(args, "-c:a:0", "aac")
			args = append(args, "-b:a:0", "128k")
		}

		// 统一走 CRF(CPU)/CQP(GPU) 质量模式, 并叠加 -maxrate 码率上限 (capped CRF/CQP)。
		// 无论源是否高码率, 各目标分辨率都走质量优先 + 码率封顶;
		// -maxrate 取值来自 OutputBitrateForTarget (按输出分辨率削减源码率)。
		val := policy.OutputCQPForTarget(t, useGPU)
		if useGPU {
			args = append(args, "-rc:v:0", "vbr", "-cq:v:0", strconv.Itoa(val))
		} else {
			args = append(args, "-crf:v:0", strconv.Itoa(val))
		}

		if vb := policy.OutputBitrateForTarget(t, sourceBitrate, sourceCodec); vb > 0 {
			args = append(args,
				"-maxrate:v:0", fmt.Sprintf("%dk", vb),
				"-bufsize:v:0", fmt.Sprintf("%dk", vb*2),
			)
		}

		// 可 seek 文件输出：+faststart 便于渐进播放。
		// 不可 seek（tcp/pipe）仍需分片 MP4。分段中间产物跳过 faststart（拼装时统一重封装）。
		output := fmt.Sprintf("pipe:%d", 3+idx)
		if idx < len(outputs) && outputs[idx] != "" {
			output = outputs[idx]
		}

		switch {
		case chunk != nil && chunk.VideoOnly:
		case isNonSeekableOutput(output):
			args = append(args, "-movflags", "frag_keyframe+empty_moov")
		default:
			args = append(args, "-movflags", "+faststart")
		}

		if chunk != nil {
			if chunk.ForceKeyframeEverySec > 0 {
				args = append(args, "-force_key_frames",
					fmt.Sprintf("expr:gte(t,n_forced*%d)", chunk.ForceKeyframeEverySec))
				if useGPU {
					args = append(args, "-forced-idr:v:0", "1")
				}
			}
			// LenSec<=0 表示末块：编到源文件结尾，吃掉 ffprobe 整数截断的小数尾巴。
			if chunk.LenSec > 0 {
				args = append(args, "-t", strconv.Itoa(chunk.LenSec))
			}
		}

		args = append(args, "-f", "mp4", output)
	}

	return args
}

// isNonSeekableOutput 判断输出目标是否不可随机写（tcp/pipe 等），用于选择 movflags。
func isNonSeekableOutput(output string) bool {
	o := strings.ToLower(strings.TrimSpace(output))

	return strings.HasPrefix(o, "tcp://") ||
		strings.HasPrefix(o, "udp://") ||
		strings.HasPrefix(o, "pipe:") ||
		o == "pipe:1" ||
		o == "-"
}

// buildDASHFFmpegArgs 构造原生分片 DASH 参数：由 FFmpeg 直接生成准确的 SegmentTimeline，
// 视频各 representation 复用 MP4 路径相同的质量/码率封顶策略，音频共享一条 AdaptationSet。
func buildDASHFFmpegArgs(
	policy *media.TranscodePolicy,
	srcPath, filter string,
	targets []media.TranscodeTarget,
	mpdPath, fileHash string,
	useGPU bool,
	sourceBitrate int,
	sourceCodec string,
	hasAudio bool,
) []string {
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}

	threads := policy.ThreadsOrZero()

	if useGPU {
		// 注意：不加 -hwaccel_output_format cuda，dash muxer 需要 CPU 侧数据；缩放仍走软件 scale。
		args = append(args, "-hwaccel", "cuda")
	}

	args = append(args, "-i", srcPath, "-filter_complex", filter)

	if threads > 0 {
		args = append(args, "-threads", strconv.Itoa(threads))
	}

	args = append(args, "-preset", policy.PresetOrDefault(useGPU))

	videoCodec := "libx264"
	if useGPU {
		videoCodec = "h264_nvenc"
	}

	// 各视频 representation (统一 CRF/CQP 质量模式 + -maxrate 码率封顶)
	for idx, t := range targets {
		args = append(args, "-map", fmt.Sprintf("[%s]", t.MapLabel))
		args = append(args, "-c:v:"+strconv.Itoa(idx), videoCodec)
		args = append(args, "-force_key_frames:v:"+strconv.Itoa(idx),
			fmt.Sprintf("expr:gte(t,n_forced*%d)", policy.SegDurationOrDefault()))
		if useGPU {
			// NVENC 默认强制 I 帧不一定是可独立解码的 IDR，DASH 需要 IDR 边界。
			args = append(args, "-forced-idr:v:"+strconv.Itoa(idx), "1")
		}

		// 根据 target 的 Quality 字段选择 CQP/CRF
		val := policy.OutputCQPForTarget(t, useGPU)
		if useGPU {
			args = append(args, "-rc:v:"+strconv.Itoa(idx), "vbr", "-cq:v:"+strconv.Itoa(idx), strconv.Itoa(val))
		} else {
			args = append(args, "-crf:v:"+strconv.Itoa(idx), strconv.Itoa(val))
		}

		if vb := policy.OutputBitrateForTarget(t, sourceBitrate, sourceCodec); vb > 0 {
			args = append(args,
				"-maxrate:v:"+strconv.Itoa(idx), fmt.Sprintf("%dk", vb),
				"-bufsize:v:"+strconv.Itoa(idx), fmt.Sprintf("%dk", vb*2),
			)
		}
	}

	// 共享单条音轨（源无音轨时跳过，否则 adaptation_sets 引用不存在的音频流会导致 ffmpeg 失败）
	adaptationSets := "id=0,streams=v"

	if hasAudio {
		args = append(args,
			"-map", "0:a:0",
			"-c:a:0", "aac",
			"-profile:a:0", "aac_low",
			"-b:a:0", "128k",
		)
		adaptationSets = "id=0,streams=v id=1,streams=a"
	}

	segDur := policy.SegDurationOrDefault()

	// 单文件模式的固定 duration 不能准确表达实际 GOP 和 AAC 尾段。
	// 使用 FFmpeg 原生时间轴；确定性相对文件名保证重试覆盖、播放器同目录取段。
	args = append(args,
		"-f", "dash",
		"-use_template", "1",
		"-use_timeline", "1",
		"-init_seg_name", fileHash+"-init-stream$RepresentationID$.mp4",
		"-media_seg_name", fileHash+"-stream$RepresentationID$-$Number%05d$.m4s",
		"-dash_segment_type", "mp4",
		"-seg_duration", strconv.Itoa(segDur),
		"-adaptation_sets", adaptationSets,
		mpdPath,
	)

	return args
}

// guessDashContentType 为 DASH 产物推断 Content-Type (供 MinIO 落对象元数据用)。
func guessDashContentType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".mpd":
		return "application/dash+xml"
	case ".m4s":
		return "video/iso.segment"
	default:
		return "video/mp4"
	}
}

// buildAudioTrackEncodeArgs 整条音轨一次性编码为 AAC-LC 128k（与两条管线原参数一致）。
func buildAudioTrackEncodeArgs(srcPath, outPath string) []string {
	return []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", srcPath,
		"-vn",
		"-map", "0:a:0",
		"-c:a", "aac",
		"-profile:a", "aac_low",
		"-b:a", "128k",
		"-f", "mp4",
		outPath,
	}
}

// writeChunkConcatList 生成 ffconcat 清单（绝对路径 + -safe 0 使用）。
func writeChunkConcatList(dir, name string, files []string) (string, error) {
	var b strings.Builder

	b.WriteString("ffconcat version 1.0\n")

	for _, f := range files {
		p := strings.ReplaceAll(filepath.ToSlash(f), "'", `'\''`)
		fmt.Fprintf(&b, "file '%s'\n", p)
	}

	listPath := filepath.Join(dir, name)
	if err := os.WriteFile(listPath, []byte(b.String()), 0o644); err != nil {
		return "", fmt.Errorf("write concat list %s: %w", name, err)
	}

	return listPath, nil
}

// buildChunkConcatMP4Args 单目标 MP4 拼装：concat 视频块 + 可选混入共享音轨 + faststart。
func buildChunkConcatMP4Args(listPath, audioPath, outPath string) []string {
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "concat", "-safe", "0", "-i", listPath,
	}

	if audioPath != "" {
		args = append(args, "-i", audioPath)
	}

	args = append(args, "-map", "0:v:0")

	if audioPath != "" {
		args = append(args, "-map", "1:a:0")
	}

	return append(args,
		"-c", "copy",
		"-movflags", "+faststart",
		"-f", "mp4",
		outPath,
	)
}

// buildChunkedDASHMuxArgs DASH 终段拼装：每个 rep 一个 concat 输入，copy 进 dash muxer。
// 块编码时已按 seg_duration 网格强制关键帧，copy 分段成立。
func buildChunkedDASHMuxArgs(listPaths []string, audioPath, mpdPath, fileHash string, segDur int) []string {
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}

	for _, lp := range listPaths {
		args = append(args, "-f", "concat", "-safe", "0", "-i", lp)
	}

	if audioPath != "" {
		args = append(args, "-i", audioPath)
	}

	for i := range listPaths {
		args = append(args, "-map", fmt.Sprintf("%d:v:0", i))
	}

	adaptationSets := "id=0,streams=v"

	if audioPath != "" {
		args = append(args, "-map", fmt.Sprintf("%d:a:0", len(listPaths)))
		adaptationSets = "id=0,streams=v id=1,streams=a"
	}

	return append(args,
		"-c", "copy",
		"-f", "dash",
		"-use_template", "1",
		"-use_timeline", "1",
		"-init_seg_name", fileHash+"-init-stream$RepresentationID$.mp4",
		"-media_seg_name", fileHash+"-stream$RepresentationID$-$Number%05d$.m4s",
		"-dash_segment_type", "mp4",
		"-seg_duration", strconv.Itoa(segDur),
		"-adaptation_sets", adaptationSets,
		mpdPath,
	)
}

// runFFmpegCopyMux 执行一次纯 copy 拼装命令（无进度输出，超时按体积估算）。
func runFFmpegCopyMux(ctx context.Context, args []string, workDir string, timeout time.Duration, label string) error {
	muxCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stderr bytes.Buffer

	cmd := exec.CommandContext(muxCtx, "ffmpeg", args...)
	if workDir != "" {
		cmd.Dir = workDir
	}

	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s copy-mux failed: %w, stderr: %s", label, err, stderr.String())
	}

	return nil
}
