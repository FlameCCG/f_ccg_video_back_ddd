package domainerr

import "errors"

// 转码任务的取消与停滞哨兵错误。
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
