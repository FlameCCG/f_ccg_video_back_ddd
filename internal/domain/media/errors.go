// Package media 表达转码计划、输出目标、任务状态与恢复规则。
//
// 先读 task.go、meta.go、policy.go，再读 plan.go/target.go 理解源片如何决定输出。
// progress.go、timeout.go 与 task_registry.go 分别管理进度、超时判定和任务取消状态。
// ports.go 声明存储、资源、进度和派发能力；FFmpeg 命令、Kafka offset 和 MinIO SDK 不进入领域。
// 规则中的高度分类与实际像素尺寸是不同概念，超宽视频、源片与高码率标签需要按目标规则理解。
//
// 本包保持纯 Go 规则与端口，不依赖 HTTP、数据库、缓存、消息框架或配置加载器。
// 领域函数尽量显式接收事实与时间，让规则可以脱离运行环境阅读和验证。
package media

import "errors"

// 转码任务的取消与停滞哨兵错误（原 service/transcode_cancel_service 与 mq_service/transcode_timeout.go）。
var (
	// ErrVideoDeleted 视频被硬删除导致转码任务作废时的取消原因。
	ErrVideoDeleted = errors.New("transcode task canceled because video was deleted")
	// ErrTranscodeProgressStalled FFmpeg out_time 长时间不前进（进度停滞看门狗触发）。
	ErrTranscodeProgressStalled = errors.New("transcode progress stalled")
	// ErrTranscodeEncodeTimeout FFmpeg 编码超过墙钟预算。
	ErrTranscodeEncodeTimeout = errors.New("transcode encode timeout")
	// ErrTranscodeUploadTimeout 单个产物上传超过按体积估算的截止时间。
	ErrTranscodeUploadTimeout = errors.New("transcode upload timeout")
)

// PermanentMessageError 标记消息应被丢弃（投递 DLQ）而不是无限重试。
type PermanentMessageError struct {
	err error
}

// PermanentMessage 包装不可恢复的消费错误。
func PermanentMessage(err error) error {
	if err == nil {
		err = errors.New("permanent message failure")
	}

	return &PermanentMessageError{err: err}
}

// Error 实现 error 接口，直接透出内部错误文本。
func (e *PermanentMessageError) Error() string {
	return e.err.Error()
}

// Unwrap 返回被包装的原始错误，供 errors.Is/As 检索。
func (e *PermanentMessageError) Unwrap() error {
	return e.err
}
