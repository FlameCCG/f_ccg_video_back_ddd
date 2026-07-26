package media

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

const (
	// 估算单文件上传超时用的最低吞吐（本地 MinIO / 局域网）。
	defaultUploadMinBytesPerSec int64 = 512 * 1024
	// UploadTimeoutBase 上传固定开销：建连 + multipart complete。
	UploadTimeoutBase = 2 * time.Minute
	// UploadTimeoutMin 上传超时下限。
	UploadTimeoutMin = 3 * time.Minute
	// UploadTimeoutMax 上传超时上限。
	UploadTimeoutMax = 2 * time.Hour
	// UploadTimeoutUnknownSize 未知体积时的固定上传预算。
	UploadTimeoutUnknownSize = 30 * time.Minute

	// EncodeTimeoutMin 编码墙钟超时下限。
	EncodeTimeoutMin = 15 * time.Minute
	// EncodeTimeoutMax 编码墙钟超时上限。
	EncodeTimeoutMax = 12 * time.Hour

	// ProgressStallMin 进度停滞判定下限。
	// 双管线 4K CPU 重编码时 out_time 可能很久才跳一截，5 分钟会误杀 → 整单 Kafka 重试、进度永远卡个位数。
	ProgressStallMin = 20 * time.Minute
	// ProgressStallMax 进度停滞判定上限。
	ProgressStallMax = 90 * time.Minute

	// DefaultDurationSec 探测失败或时长为 0 时的兜底时长（按约 10 分钟片源估算）。
	DefaultDurationSec = 600

	// 拼装（-c copy）超时估算：本地磁盘 I/O 下限吞吐 + 固定开销。
	assembleMinBytesPerSec int64 = 10 * 1024 * 1024
	// AssembleTimeoutBase 拼装固定开销。
	AssembleTimeoutBase = 2 * time.Minute
	// AssembleTimeoutMin 拼装超时下限。
	AssembleTimeoutMin = 5 * time.Minute
	// AssembleTimeoutMax 拼装超时上限。
	AssembleTimeoutMax = 1 * time.Hour

	// AudioEncodeTimeoutMin 音频整轨编码超时下限（AAC 编码约百倍实时，预算给足冗余）。
	AudioEncodeTimeoutMin = 10 * time.Minute
	// AudioEncodeTimeoutMax 音频整轨编码超时上限。
	AudioEncodeTimeoutMax = 1 * time.Hour
)

// ProgressCallback 接收 FFmpeg 解码出的媒体 out_time（秒）。
type ProgressCallback func(outTimeSec float64)

// CalcPipelineEncodeTimeout 按片源时长、阶梯路数、GPU/CPU 模式，以及可选的源文件体积
// 估算 FFmpeg 编码墙钟预算（体积越大给的余量越多）。
//
// 多输出单次 pass 约等于一次解码 + N 路编码；CPU 4K medium 可能远慢于实时。
// 倍率故意偏保守，保证正常慢编码能跑完；真死锁靠进度停滞看门狗更早发现。
func CalcPipelineEncodeTimeout(durationSec, targetCount int, useGPU bool, sourceBytes int64) time.Duration {
	if durationSec <= 0 {
		durationSec = DefaultDurationSec
	}

	if targetCount < 1 {
		targetCount = 1
	}

	// 单次多输出 pass 的基础实时倍率。
	mult := 20 // CPU
	if useGPU {
		mult = 6
	}
	// 同进程输出路数越多，滤镜/编码成本越高。
	if targetCount > 2 {
		mult += (targetCount - 2) * 2
	}

	d := time.Duration(durationSec*mult) * time.Second

	// 源体积加成：约每 50MiB 多给 1 分钟（重 4K 源需要更大预算）。
	if sourceBytes > 0 {
		d += time.Duration(sourceBytes/(50*1024*1024)) * time.Minute
	}

	if d < EncodeTimeoutMin {
		return EncodeTimeoutMin
	}

	if d > EncodeTimeoutMax {
		return EncodeTimeoutMax
	}

	return d
}

// CalcProgressStallTimeout 为 FFmpeg out_time 最长允许静默时间，超时则杀进程。
// 随片长缩放；下限较高，避免重 4K 双管线 CPU 编码被误杀后 Kafka 反复重投。
func CalcProgressStallTimeout(durationSec int) time.Duration {
	if durationSec <= 0 {
		durationSec = DefaultDurationSec
	}
	// 约 duration/3，例如 25 分钟片源 ≈ 8 分钟 → 抬到下限 20 分钟。
	d := time.Duration(durationSec) * time.Second / 3
	if d < ProgressStallMin {
		return ProgressStallMin
	}

	if d > ProgressStallMax {
		return ProgressStallMax
	}

	return d
}

// CalcUploadTimeout 按文件大小估算单对象上传截止时间。
func CalcUploadTimeout(fileSize int64) time.Duration {
	if fileSize <= 0 {
		// 未知大小：给固定宽裕预算（落盘输出正常不应走到这里）。
		return UploadTimeoutUnknownSize
	}

	bps := defaultUploadMinBytesPerSec

	secs := fileSize / bps
	if fileSize%bps != 0 {
		secs++
	}

	d := time.Duration(secs)*time.Second + UploadTimeoutBase

	if d < UploadTimeoutMin {
		return UploadTimeoutMin
	}

	if d > UploadTimeoutMax {
		return UploadTimeoutMax
	}

	return d
}

// CalcAssembleTimeout 按待拼装总字节数估算 -c copy 拼装超时。
func CalcAssembleTimeout(totalBytes int64) time.Duration {
	if totalBytes <= 0 {
		return AssembleTimeoutMin
	}

	secs := totalBytes / assembleMinBytesPerSec
	if totalBytes%assembleMinBytesPerSec != 0 {
		secs++
	}

	d := time.Duration(secs)*time.Second + AssembleTimeoutBase
	if d < AssembleTimeoutMin {
		return AssembleTimeoutMin
	}

	if d > AssembleTimeoutMax {
		return AssembleTimeoutMax
	}

	return d
}

// CalcAudioEncodeTimeout 按片长估算共享音轨整轨编码超时。
func CalcAudioEncodeTimeout(durationSec int) time.Duration {
	if durationSec <= 0 {
		durationSec = DefaultDurationSec
	}

	d := time.Duration(durationSec/5)*time.Second + 2*time.Minute
	if d < AudioEncodeTimeoutMin {
		return AudioEncodeTimeoutMin
	}

	if d > AudioEncodeTimeoutMax {
		return AudioEncodeTimeoutMax
	}

	return d
}

// StallObserver 在判定进度停滞时被调用，供基础设施层记录日志。
type StallObserver func(idle, limit time.Duration)

// WithProgressStallGuard 在 FFmpeg 进度（out_time）停止前进时取消 ctx。
// 请把返回的 wrap 包在真实 progress 回调外，每次 tick 刷新计时。
//
// onStall 可为 nil；非 nil 时在取消前回调一次，便于基础设施层打日志。
func WithProgressStallGuard(
	parent context.Context,
	stallLimit time.Duration,
	onStall StallObserver,
) (ctx context.Context, cancel context.CancelFunc, wrap func(ProgressCallback) ProgressCallback) {
	ctx, cancelCause := context.WithCancelCause(parent)

	var last atomic.Int64

	last.Store(time.Now().UnixNano())

	wrap = func(inner ProgressCallback) ProgressCallback {
		return func(outTimeSec float64) {
			last.Store(time.Now().UnixNano())

			if inner != nil {
				inner(outTimeSec)
			}
		}
	}

	cancel = func() {
		cancelCause(nil)
	}

	if stallLimit <= 0 {
		return ctx, cancel, wrap
	}

	// 轮询间隔：既要尽快发现停滞，又避免忙等。
	// 生产环境限制多为分钟级 → 约 15s 一跳；测试用短限制会更快。
	interval := stallLimit / 3
	if interval < 50*time.Millisecond {
		interval = 50 * time.Millisecond
	}

	if interval > 15*time.Second {
		interval = 15 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				idle := time.Since(time.Unix(0, last.Load()))
				if idle > stallLimit {
					if onStall != nil {
						onStall(idle.Round(time.Second), stallLimit)
					}

					cancelCause(fmt.Errorf("%w: no out_time for %s (limit %s)",
						ErrTranscodeProgressStalled, idle.Round(time.Second), stallLimit))

					return
				}
			}
		}
	}()

	return ctx, cancel, wrap
}

// ClassifyFFmpegCtxError 把 context 取消原因映射为稳定的管线错误。
func ClassifyFFmpegCtxError(runCtx context.Context, waitErr error, encodeTimeout time.Duration, stderr string) error {
	if waitErr == nil {
		return nil
	}

	cause := context.Cause(runCtx)
	if errors.Is(cause, ErrTranscodeProgressStalled) {
		return fmt.Errorf("ffmpeg pipeline failed: %w, stderr: %s", cause, stderr)
	}

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) || errors.Is(cause, context.DeadlineExceeded) {
		return fmt.Errorf("ffmpeg pipeline failed: %w after %s: %v, stderr: %s",
			ErrTranscodeEncodeTimeout, encodeTimeout, waitErr, stderr)
	}

	if cause != nil && !errors.Is(cause, context.Canceled) {
		return fmt.Errorf("ffmpeg pipeline failed: %w (%v), stderr: %s", waitErr, cause, stderr)
	}

	return fmt.Errorf("ffmpeg pipeline failed: %w, stderr: %s", waitErr, stderr)
}
