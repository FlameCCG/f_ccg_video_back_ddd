package ai

import (
	"context"
)

// ImageInvocation 图片生成调用参数。
type ImageInvocation struct {
	Runtime        RuntimeConfig
	Model          string
	Prompt         string
	N              *int
	AspectRatio    string
	Resolution     string
	ResponseFormat string
	Size           string
	// Refs 参考图地址列表；为空表示纯文生图。
	Refs []string
}

// ImageGenerator 文生图/图生图端口（上游为方舟 Ark 图片模型）。
type ImageGenerator interface {
	// Generate 生成图片并返回上游兼容的响应载荷。
	Generate(ctx context.Context, inv ImageInvocation) (any, error)
}
