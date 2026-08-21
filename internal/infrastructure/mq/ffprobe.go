package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	media "f_ccg_video_back_ddd/internal/domain/media"

	"go.uber.org/zap"
)

// ffprobeStream 是 ffprobe -show_streams 的单条流结构。
type ffprobeStream struct {
	CodecType string `json:"codec_type"`
	Height    int    `json:"height"`
	Width     int    `json:"width"`
	BitRate   string `json:"bit_rate"`
	Codec     string `json:"codec_name"`
	Profile   string `json:"profile"`
	PixFmt    string `json:"pix_fmt"`
	Level     int    `json:"level"`
	Duration  string `json:"duration"`
}

// ffprobeResult 是 ffprobe -show_streams -show_format 的输出结构。
type ffprobeResult struct {
	Streams []ffprobeStream `json:"streams"`
	Format  struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		Size       string `json:"size"`
		BitRate    string `json:"bit_rate"`
		Tags       struct {
			MajorBrand       string `json:"major_brand"`
			CompatibleBrands string `json:"compatible_brands"`
		} `json:"tags"`
	} `json:"format"`
}

// ProbeVideoMeta 探测本地视频文件元数据。
func ProbeVideoMeta(ctx context.Context, p string, logger *zap.Logger) (media.VideoMeta, error) {
	return ProbeVideo(ctx, p, logger)
}

// ProbeVideo 调用 ffprobe 解析视频元数据（分辨率、码率、编码、容器 brand、时长等）。
func ProbeVideo(ctx context.Context, p string, logger *zap.Logger) (media.VideoMeta, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_streams",
		"-show_format",
		"-of", "json",
		p,
	)

	out, err := cmd.Output()
	if err != nil {
		return media.VideoMeta{}, fmt.Errorf("ffprobe failed: %w", err)
	}

	var result ffprobeResult
	if err := json.Unmarshal(out, &result); err != nil {
		logger.Error("unmarshal ffprobe json failed", zap.String("json", string(out)))

		return media.VideoMeta{}, err
	}

	if len(result.Streams) == 0 {
		return media.VideoMeta{}, errors.New("no video stream found")
	}

	var videoStream, audioStream *ffprobeStream

	for i := range result.Streams {
		switch result.Streams[i].CodecType {
		case "video":
			if videoStream == nil {
				videoStream = &result.Streams[i]
			}
		case "audio":
			if audioStream == nil {
				audioStream = &result.Streams[i]
			}
		}
	}

	if videoStream == nil {
		return media.VideoMeta{}, errors.New("no video stream found")
	}

	format := result.Format

	v, err := strconv.ParseFloat(videoStream.Duration, 64)
	if err != nil || v <= 0 {
		v, err = strconv.ParseFloat(format.Duration, 64)
		if err != nil || v <= 0 {
			logger.Warn("无效的视频时长信息")

			v = 0
		}
	}

	meta := media.VideoMeta{
		Height:           videoStream.Height,
		Width:            videoStream.Width,
		Format:           format.FormatName,
		Codec:            strings.TrimSpace(videoStream.Codec),
		VideoProfile:     strings.TrimSpace(videoStream.Profile),
		PixelFormat:      strings.TrimSpace(videoStream.PixFmt),
		VideoLevel:       videoStream.Level,
		MajorBrand:       strings.TrimSpace(format.Tags.MajorBrand),
		CompatibleBrands: strings.TrimSpace(format.Tags.CompatibleBrands),
		Duration:         int(v),
	}

	if audioStream != nil {
		meta.AudioCodec = strings.TrimSpace(audioStream.Codec)
		meta.AudioProfile = strings.TrimSpace(audioStream.Profile)
	}

	if size, err := strconv.ParseInt(format.Size, 10, 64); err == nil && size > 0 {
		meta.Size = size
	}

	br, _ := strconv.Atoi(format.BitRate)
	if br <= 0 {
		br, _ = strconv.Atoi(videoStream.BitRate)
	}

	if br <= 0 {
		size, _ := strconv.ParseInt(format.Size, 10, 64)

		dur, _ := strconv.ParseFloat(format.Duration, 64)
		if dur <= 0 {
			dur, _ = strconv.ParseFloat(videoStream.Duration, 64)
		}

		if size > 0 && dur > 0 {
			bitrateBps := float64(size*8) / dur
			br = int(bitrateBps)
		}
	}

	if br > 0 {
		meta.Bitrate = br / 1000
	}

	return meta, nil
}

// probeContainerDurationSec 探测容器时长（音频文件无视频流，不能走 ProbeVideo）。
func probeContainerDurationSec(ctx context.Context, p string) (int, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "json",
		p,
	)

	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe duration failed: %w", err)
	}

	var result struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}

	if err := json.Unmarshal(out, &result); err != nil {
		return 0, err
	}

	v, err := strconv.ParseFloat(result.Format.Duration, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid container duration %q", result.Format.Duration)
	}

	return int(v), nil
}

// sourceBytesHint 返回本地源文件大小，供超时预算使用；不可用时返回 0。
func sourceBytesHint(srcPath string) int64 {
	if srcPath == "" {
		return 0
	}

	fi, err := os.Stat(srcPath)
	if err != nil || fi.IsDir() {
		return 0
	}

	return fi.Size()
}

// sumFileSizes 累加若干本地文件的字节数（目录与不存在的路径跳过）。
func sumFileSizes(paths ...string) int64 {
	var total int64

	for _, p := range paths {
		if p == "" {
			continue
		}

		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			total += info.Size()
		}
	}

	return total
}
