package video

import (
	"time"
)

// UploadStatus 分片上传状态。
type UploadStatus struct {
	FileHash       string
	Completed      bool
	FilePath       string
	UploadedChunks []string
}

// TranscodeProgressItem 转码进度项，JSON 会直接写入 SSE data 与轮询响应，字段名属于外部契约。
type TranscodeProgressItem struct {
	VideoID      uint                        `json:"videoId"`
	PartID       uint                        `json:"partId,omitempty"`
	Status       string                      `json:"status"`
	Percent      float64                     `json:"percent"`
	Stage        string                      `json:"stage"`
	StagePercent float64                     `json:"stagePercent"`
	Message      string                      `json:"message,omitempty"`
	DurationSec  int                         `json:"durationSec,omitempty"`
	OutTimeSec   float64                     `json:"outTimeSec,omitempty"`
	Error        string                      `json:"error,omitempty"`
	UpdatedAt    time.Time                   `json:"updatedAt"`
	StartedAt    time.Time                   `json:"startedAt,omitempty"`
	Pipelines    []TranscodePipelineProgress `json:"pipelines,omitempty"`
}

// TranscodePipelineProgress 单个 FFmpeg 管线的进度。
type TranscodePipelineProgress struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Percent     float64   `json:"percent"`
	Message     string    `json:"message,omitempty"`
	Error       string    `json:"error,omitempty"`
	Resolutions []uint16  `json:"resolutions,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
