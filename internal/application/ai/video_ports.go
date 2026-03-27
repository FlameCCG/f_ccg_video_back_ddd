package ai

import (
	"context"
)

// VideoInvocation 视频生成任务参数。
type VideoInvocation struct {
	Runtime       RuntimeConfig
	Model         string
	Prompt        string
	Duration      *int
	Seconds       *int
	AspectRatio   string
	Resolution    string
	Size          string
	ImageRefs     []VideoImageRef
	GenerateAudio *bool
	CameraFixed   *bool
	Watermark     *bool
	Seed          *int
}

// VideoImageRef 保留首帧和参考图的用途，避免在协议转换前丢失生成模式。
type VideoImageRef struct {
	URL  string
	Role string
}

const (
	VideoImageFirstFrame = "first_frame"
	VideoImageReference  = "reference_image"
)

// VideoTaskClient 视频生成任务端口（上游为方舟 runtime SDK）。
type VideoTaskClient interface {
	// Create 创建视频生成任务。
	Create(ctx context.Context, inv VideoInvocation) (any, error)
	// Get 查询视频生成任务状态。
	Get(ctx context.Context, runtime RuntimeConfig, requestID string) (any, error)
}
