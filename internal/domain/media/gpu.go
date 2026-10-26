package media

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"
)

// GPU 失败冷却：一般故障 10 分钟；驱动/NVENC 版本不匹配等「不升级就不会好」的故障拉长，
// 避免每条任务/每次冷却结束都先空跑 GPU 再回退 CPU，拖慢并放大重试噪声。
const (
	// GPUFailureCooldown 一般 GPU 故障的冷却时长。
	GPUFailureCooldown = 10 * time.Minute
	// GPUFatalDriverCooldown 驱动/API 版本不匹配的长冷却时长。
	GPUFatalDriverCooldown = 6 * time.Hour
)

// GPUCooldown 跟踪 GPU 编码的临时禁用窗口。
//
// 原实现为 mq_service 的包级变量 gpuUnavailableUntil；
// 新架构改为可注入结构体字段，进程内共享一个实例即可保持同样的跨任务冷却语义。
type GPUCooldown struct {
	until atomic.Int64
}

// Active 返回当前冷却截止时间与是否仍在冷却中；已过期会顺手清零。
func (c *GPUCooldown) Active() (time.Time, bool) {
	if c == nil {
		return time.Time{}, false
	}

	untilUnix := c.until.Load()
	if untilUnix <= 0 {
		return time.Time{}, false
	}

	until := time.Unix(0, untilUnix)
	if time.Now().After(until) {
		c.until.Store(0)

		return time.Time{}, false
	}

	return until, true
}

// NoteFailure 依据 GPU 失败错误决定是否进入冷却。
// 返回冷却截止时间、冷却时长，以及是否真的进入了冷却。
func (c *GPUCooldown) NoteFailure(err error) (time.Time, time.Duration, bool) {
	if c == nil || !IsFatalGPUFailure(err) {
		return time.Time{}, 0, false
	}

	cooldown := GPUFailureCooldown
	if IsFatalGPUDriverMismatch(err) {
		cooldown = GPUFatalDriverCooldown
	}

	until := time.Now().Add(cooldown)
	c.until.Store(until.UnixNano())

	return until, cooldown, true
}

// IsFatalGPUFailure 判断错误是否属于「GPU 侧不可用」类故障。
func IsFatalGPUFailure(err error) bool {
	if err == nil {
		return false
	}

	message := lowerTrim(err.Error())

	return strings.Contains(message, "signal: killed") ||
		strings.Contains(message, "cuda") ||
		strings.Contains(message, "nvenc") ||
		strings.Contains(message, "no capable devices found") ||
		strings.Contains(message, "cannot init encoder") ||
		strings.Contains(message, "device busy")
}

// IsFatalGPUDriverMismatch 驱动/API 版本不够：冷却期内再试必失败，应长冷却。
func IsFatalGPUDriverMismatch(err error) bool {
	if err == nil {
		return false
	}

	message := lowerTrim(err.Error())

	return strings.Contains(message, "nvenc api version") ||
		strings.Contains(message, "driver does not support") ||
		strings.Contains(message, "minimum required nvidia driver")
}

// ClassifyInterruptReason 把任务中断原因归类为稳定的字符串标签，供日志排查。
//
// sessionCanceled 表示 Kafka 消费会话是否已取消（rebalance / 关机）。
func ClassifyInterruptReason(taskCtx context.Context, err error, sessionCanceled bool) string {
	if err == nil && taskCtx != nil {
		err = context.Cause(taskCtx)
	}

	causeIs := func(target error) bool {
		return taskCtx != nil && errors.Is(context.Cause(taskCtx), target)
	}

	return finishClassifyInterruptReason(taskCtx, err, sessionCanceled, causeIs)
}

// TaskWasDeleted 报告任务是否因视频被删除而作废。
func TaskWasDeleted(ctx context.Context, err error) bool {
	if ctx != nil && errors.Is(context.Cause(ctx), ErrVideoDeleted) {
		return true
	}

	return errors.Is(err, ErrVideoDeleted)
}

func finishClassifyInterruptReason(taskCtx context.Context, err error, sessionCanceled bool, causeIs func(error) bool) string {
	for _, rule := range []struct {
		target error
		reason string
	}{
		{ErrTranscodeProgressStalled, "progress_stalled"}, {ErrTranscodeEncodeTimeout, "encode_timeout"}, {ErrTranscodeUploadTimeout, "upload_timeout"}, {ErrVideoDeleted, "video_deleted"},
	} {
		if errors.Is(err, rule.target) || causeIs(rule.target) {
			return rule.reason
		}
	}

	if sessionCanceled {
		return "kafka_session_canceled"
	}

	for _, rule := range []struct {
		target error
		reason string
	}{{context.Canceled, "context_canceled"}, {context.DeadlineExceeded, "deadline_exceeded"}} {
		if errors.Is(err, rule.target) || (taskCtx != nil && errors.Is(taskCtx.Err(), rule.target)) {
			return rule.reason
		}
	}

	return "process_error"
}
