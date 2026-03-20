package ai

import (
	"encoding/json"
)

// VideoGenerationRequest 表示视频生成请求参数.
type VideoGenerationRequest struct {
	Model           string          `json:"model,omitempty"`
	Prompt          string          `json:"prompt,omitempty"`
	Duration        *int            `json:"duration,omitempty"`
	Seconds         *int            `json:"seconds,omitempty"`
	AspectRatio     string          `json:"aspect_ratio,omitempty"`
	Resolution      string          `json:"resolution,omitempty"`
	Image           *ImageRef       `json:"image,omitempty"`
	InputReference  *ImageRef       `json:"input_reference,omitempty"`
	ReferenceImages []ImageRef      `json:"reference_images,omitempty"`
	GenerateAudio   *bool           `json:"generate_audio,omitempty"`
	CameraFixed     *bool           `json:"camera_fixed,omitempty"`
	Watermark       *bool           `json:"watermark,omitempty"`
	Seed            *int            `json:"seed,omitempty"`
	Output          json.RawMessage `json:"output,omitempty"`
	User            string          `json:"user,omitempty"`
	Size            string          `json:"size,omitempty"`
}
