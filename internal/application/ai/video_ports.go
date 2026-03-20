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
	ImageRefs     []string
	GenerateAudio *bool
	CameraFixed   *bool
	Watermark     *bool
	Seed          *int
}

// VideoTaskClient 视频生成任务端口（上游为方舟 runtime SDK）。
type VideoTaskClient interface {
	// Create 创建视频生成任务。
	Create(ctx context.Context, inv VideoInvocation) (any, error)
	// Get 查询视频生成任务状态。
	Get(ctx context.Context, runtime RuntimeConfig, requestID string) (any, error)
}
