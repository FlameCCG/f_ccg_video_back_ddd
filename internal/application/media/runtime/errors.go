package runtime

import "errors"

// 转码超时与停滞哨兵。错误文案是外部契约，分类结果决定重试、取消和日志标签。
var (
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
