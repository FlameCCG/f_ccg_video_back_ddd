package media

import (
	"time"

	"f_ccg_video_back_ddd/internal/config"
	domain "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// TranscodeTaskInput 是投递转码任务的入参（字段与 Kafka 消息体 pkg/mq.TranscodePayload 一一对应）。
//
// video 上下文经此结构投递任务，避免跨上下文共享领域实体。
type TranscodeTaskInput struct {
	// ResolutionList 投稿时配置的目标分辨率阶梯。
	ResolutionList []uint16
	// StorageType 源文件所在存储类型。
	StorageType enum.VideoStorageType
	// SourcePath 源文件路径（本地绝对路径或可访问 URL）。
	SourcePath string
	// OutputPath 产物在存储中的相对目录前缀。
	OutputPath string
	// FileHash 上传时的文件指纹。
	FileHash string
	// FileName 用户上传时的原始文件名。
	FileName string
	// VideoID 所属视频 ID。
	VideoID uint
	// VideoPartID 所属分P ID；为 0 表示单P视频。
	VideoPartID uint
}

// task 把入参转成领域任务实体。
func (in TranscodeTaskInput) task() domain.TranscodeTask {
	return domain.TranscodeTask{
		ResolutionList: in.ResolutionList,
		StorageType:    in.StorageType,
		SourcePath:     in.SourcePath,
		OutputPath:     in.OutputPath,
		FileHash:       in.FileHash,
		FileName:       in.FileName,
		VideoID:        in.VideoID,
		VideoPartID:    in.VideoPartID,
	}
}

// TranscodeProgressItem 是转码进度读模型。
//
// json tag 与原 service/mq_service.TranscodeProgress 逐字一致：
// 管理端进度接口与 SSE 直接序列化该结构，字段名属于外部契约。
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

// TranscodePipelineProgress 是单条 FFmpeg 管线的进度读模型。
type TranscodePipelineProgress struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Percent     float64   `json:"percent"`
	Message     string    `json:"message,omitempty"`
	Error       string    `json:"error,omitempty"`
	Resolutions []uint16  `json:"resolutions,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// progressItemFrom 把领域进度快照转成读模型。
func progressItemFrom(p domain.TranscodeProgress) TranscodeProgressItem {
	item := TranscodeProgressItem{
		VideoID:      p.VideoID,
		PartID:       p.PartID,
		Status:       p.Status,
		Percent:      p.Percent,
		Stage:        p.Stage,
		StagePercent: p.StagePercent,
		Message:      p.Message,
		DurationSec:  p.DurationSec,
		OutTimeSec:   p.OutTimeSec,
		Error:        p.Error,
		UpdatedAt:    p.UpdatedAt,
		StartedAt:    p.StartedAt,
	}

	if len(p.Pipelines) == 0 {
		return item
	}

	item.Pipelines = make([]TranscodePipelineProgress, 0, len(p.Pipelines))
	for _, pipeline := range p.Pipelines {
		item.Pipelines = append(item.Pipelines, TranscodePipelineProgress{
			Name:        pipeline.Name,
			Status:      pipeline.Status,
			Percent:     pipeline.Percent,
			Message:     pipeline.Message,
			Error:       pipeline.Error,
			Resolutions: append([]uint16(nil), pipeline.Resolutions...),
			UpdatedAt:   pipeline.UpdatedAt,
		})
	}

	return item
}

// PolicyFromConfig 把转码配置转换为领域策略值对象。
//
// cfg 为 nil 时返回 nil：领域侧所有 *TranscodePolicy 方法对 nil 接收者安全，
// 回落值与原实现「global.Config().Transcode == nil」分支逐一对应。
func PolicyFromConfig(cfg *config.Transcode) *domain.TranscodePolicy {
	if cfg == nil {
		return nil
	}

	return &domain.TranscodePolicy{
		MaxWorkers:           cfg.MaxWorkers,
		TranscodeResolutions: append([]uint16(nil), cfg.TranscodeResolutions...),
		HighBitrateThreshold: cfg.HighBitrateThreshold,

		Bitrate2160HighKbpsReduceRatio: cfg.Bitrate2160HighKbpsReduceRatio,
		Bitrate1080HighKbpsReduceRatio: cfg.Bitrate1080HighKbpsReduceRatio,
		Bitrate2160KbpsReduceRatio:     cfg.Bitrate2160KbpsReduceRatio,
		Bitrate1080KbpsReduceRatio:     cfg.Bitrate1080KbpsReduceRatio,
		Bitrate720KbpsReduceRatio:      cfg.Bitrate720KbpsReduceRatio,
		Bitrate360KbpsReduceRatio:      cfg.Bitrate360KbpsReduceRatio,

		CrfHigh:      cfg.CrfHigh,
		CrfMedium:    cfg.CrfMedium,
		Crf720:       cfg.Crf720,
		Crf360:       cfg.Crf360,
		GpuCQPHigh:   cfg.GpuCQPHigh,
		GpuCQPMedium: cfg.GpuCQPMedium,
		GpuCQP720:    cfg.GpuCQP720,
		GpuCQP360:    cfg.GpuCQP360,

		CpuMode:       cfg.CpuMode,
		GpuMode:       cfg.GpuMode,
		CpuPreset:     cfg.CpuPreset,
		GpuPreset:     cfg.GpuPreset,
		Threads:       cfg.Threads,
		HardwareScale: cfg.HardwareScale,

		Mp4Enable:       cfg.Mp4Enable,
		DashEnable:      cfg.DashEnable,
		DashSegDuration: cfg.DashSegDuration,

		CheckpointDir:     cfg.CheckpointDir,
		CheckpointDisable: cfg.CheckpointDisable,
		ChunkDurationSec:  cfg.ChunkDurationSec,
	}
}
