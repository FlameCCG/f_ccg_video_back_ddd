package media

import "fmt"

// -----------------------------------------------------------------------------
// 分段转码（chunked encode）：断点续传的「块级」粒度
//
// 长视频按固定块长切分，每块独立跑一次多输出 FFmpeg（纯视频），产物按块提交断点；
// 中断重试从第一个未完成的块继续，最多丢「正在编的那一个块」，而不是整次编码。
//
// 设计要点：
//   - 切块计划由 (片长, 块长) 确定性推导，重试无需额外状态即可重建同一计划；
//     块长对齐 DASH seg_duration 网格，保证 DASH copy 拼装时关键帧仍落在分段边界。
//   - 视频块用 `-ss <start>`(精确解码定位) + `-t <len>` 编码，音频不进块：
//     整条音轨单独一次性编码为共享 audio.m4a，终段拼装时混入 —— 彻底绕开
//     「逐块编 AAC 再 concat」在拼接点的爆音/缝隙问题。
//   - 块目录按编码模式(gpu/cpu)隔离：NVENC 与 libx264 的参数集不同，绝不允许混拼。
// -----------------------------------------------------------------------------

const (
	// DefaultTranscodeChunkSec 默认块长（秒）。
	DefaultTranscodeChunkSec = 180
	// MinTranscodeChunkSec 块长下限：太碎会放大每块的进程启动/seek 固定开销。
	MinTranscodeChunkSec = 60
	// TranscodeAudioTrackName 共享音轨在断点目录中的提交名（MP4 与 DASH 共用）。
	TranscodeAudioTrackName = "audio.m4a"
)

// TranscodeChunk 描述一个确定性切块（源片时间轴上的半开区间 [StartSec, StartSec+LenSec)）。
type TranscodeChunk struct {
	// Index 块序号，从 0 开始。
	Index int
	// StartSec 块起点（秒）。
	StartSec int
	// LenSec 块长度（秒）。
	LenSec int
}

// FFmpegChunkSpan 让 ffmpeg 参数构造产出「只编某个时间段」的参数。
type FFmpegChunkSpan struct {
	// StartSec 块起点；>0 时输入侧 -ss 精确 seek。
	StartSec int
	// LenSec 块长度；<=0 表示编到源文件结尾（末块）。
	LenSec int
	// VideoOnly 为 true 时不 map 音频（块级编码音频单独走共享音轨）。
	VideoOnly bool
	// ForceKeyframeEverySec >0 时按秒强制关键帧（DASH 块需要，保持全局 seg 网格）。
	ForceKeyframeEverySec int
}

// PlanTranscodeChunks 确定性切块：块长归一到 segDur 整数倍，短片(不足 2 块)不分块，
// 过短尾块并入前一块。返回 nil 表示不分块（走单次直跑）。
func PlanTranscodeChunks(durationSec, segDurSec, chunkLenSec int) []TranscodeChunk {
	if durationSec <= 0 {
		return nil
	}

	if segDurSec <= 0 {
		segDurSec = defaultDashSegDuration
	}

	chunkLen := chunkLenSec
	if chunkLen <= 0 {
		chunkLen = DefaultTranscodeChunkSec
	}

	if chunkLen < MinTranscodeChunkSec {
		chunkLen = MinTranscodeChunkSec
	}

	if rem := chunkLen % segDurSec; rem != 0 {
		chunkLen += segDurSec - rem
	}

	if durationSec < 2*chunkLen {
		return nil
	}

	chunks := make([]TranscodeChunk, 0, durationSec/chunkLen+1)

	for start := 0; start < durationSec; start += chunkLen {
		length := chunkLen
		if start+length > durationSec {
			length = durationSec - start
		}

		chunks = append(chunks, TranscodeChunk{Index: len(chunks), StartSec: start, LenSec: length})
	}

	if n := len(chunks); n > 1 && chunks[n-1].LenSec < 2*segDurSec {
		chunks[n-2].LenSec += chunks[n-1].LenSec
		chunks = chunks[:n-1]
	}

	return chunks
}

// ChunkPlan 运行时切块入口：断点不可跨尝试复用（ephemeral / 未开启）或配置关闭时返回 nil。
//
// checkpointPersistent 表示断点目录是否可跨尝试复用；为 false 时分段没有意义。
func (p *TranscodePolicy) ChunkPlan(sourceDurationSec int, checkpointPersistent bool) []TranscodeChunk {
	if !checkpointPersistent {
		return nil
	}

	segDur := defaultDashSegDuration
	chunkLen := 0

	if p != nil {
		if p.DashSegDuration > 0 {
			segDur = p.DashSegDuration
		}

		if p.ChunkDurationSec < 0 {
			return nil
		}

		chunkLen = p.ChunkDurationSec
	}

	return PlanTranscodeChunks(sourceDurationSec, segDur, chunkLen)
}

// ChunkArtifactDirName 块产物提交目录名；编码模式(gpu/cpu)进入命名空间，禁止跨编码器混拼。
func ChunkArtifactDirName(pipeline string, useGPU bool, index int) string {
	mode := "cpu"
	if useGPU {
		mode = "gpu"
	}

	return fmt.Sprintf("%s-chunk-%s-%04d", pipeline, mode, index)
}

// IsLastChunk 报告 chunk 是否为计划中的末块（末块编到源结尾，不加 -t）。
func IsLastChunk(plan []TranscodeChunk, chunk TranscodeChunk) bool {
	if len(plan) == 0 {
		return false
	}

	return chunk.Index == plan[len(plan)-1].Index
}
