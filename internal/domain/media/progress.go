package media

import (
	"time"
)

// 转码任务在 Redis 中的状态取值（外部契约，逐字保持）。
const (
	// TranscodeStatusQueued 已投递、等待消费。
	TranscodeStatusQueued = "queued"
	// TranscodeStatusRunning 正在转码。
	TranscodeStatusRunning = "running"
	// TranscodeStatusSucceeded 转码成功。
	TranscodeStatusSucceeded = "succeeded"
	// TranscodeStatusFailed 转码失败。
	TranscodeStatusFailed = "failed"
)

// 转码管线阶段（进度上报用）。
const (
	// TranscodeStageQueued 排队中。
	TranscodeStageQueued = "queued"
	// TranscodeStagePrepare 准备源文件。
	TranscodeStagePrepare = "prepare"
	// TranscodeStageProbe 探测元信息。
	TranscodeStageProbe = "probe"
	// TranscodeStageSaveOriginal 保存源视频资源。
	TranscodeStageSaveOriginal = "save_original"
	// TranscodeStageNormalize 源片规范化。
	TranscodeStageNormalize = "normalize"
	// TranscodeStageMultiRes 多分辨率转码。
	TranscodeStageMultiRes = "multi_res"
	// TranscodeStageDash DASH 产出。
	TranscodeStageDash = "dash"
	// TranscodeStageCleanup 清理与缓存刷新。
	TranscodeStageCleanup = "cleanup"
)

// TranscodeProgress 是单个视频/分P 任务的进度快照（Redis 中存储的结构）。
type TranscodeProgress struct {
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

// TranscodePipelineProgress 描述一条 FFmpeg 管线的进度（共享时间轴，无单输出百分比）。
type TranscodePipelineProgress struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Percent     float64   `json:"percent"`
	Message     string    `json:"message,omitempty"`
	Error       string    `json:"error,omitempty"`
	Resolutions []uint16  `json:"resolutions,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Terminal 报告进度是否已进入终态。
func (p TranscodeProgress) Terminal() bool {
	return p.Status == TranscodeStatusSucceeded || p.Status == TranscodeStatusFailed
}

// Normalize 在写入前夹紧百分比区间并刷新更新时间。
func (p *TranscodeProgress) Normalize(now time.Time) {
	if p == nil {
		return
	}

	if p.Percent < 0 {
		p.Percent = 0
	}

	if p.Percent > 100 {
		p.Percent = 100
	}

	p.UpdatedAt = now
}

// NewQueuedProgress 构造「已投递、排队等待转码」的初始进度快照。
func NewQueuedProgress(videoID, partID uint, now time.Time) *TranscodeProgress {

	return &TranscodeProgress{
		VideoID:   videoID,
		PartID:    partID,
		Status:    TranscodeStatusQueued,
		Percent:   0,
		Stage:     TranscodeStageQueued,
		Message:   "排队等待转码",
		UpdatedAt: now,
		StartedAt: now,
	}
}
