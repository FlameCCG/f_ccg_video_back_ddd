package video

import (
	app "f_ccg_video_back_ddd/internal/application/video"

	"go.uber.org/zap"
)

// AdminHandler 视频管理端 HTTP 处理器
// （视频列表/详情、审核、恢复、删除、举报处理、弹幕管理、分区管理、转码进度）。
type AdminHandler struct {
	admin     *app.AdminVideoService
	detail    *app.DetailService
	danmu     *app.DanmuService
	reports   *app.ReportService
	partition *app.PartitionService
	progress  app.TranscodeProgressReader
	vectors   *app.VectorSyncService
	logger    *zap.Logger
}

// NewAdminHandler 构造视频管理端处理器。
func NewAdminHandler(
	admin *app.AdminVideoService,
	detail *app.DetailService,
	danmu *app.DanmuService,
	reports *app.ReportService,
	partition *app.PartitionService,
	progress app.TranscodeProgressReader,
	vectors *app.VectorSyncService,
	logger *zap.Logger,
) *AdminHandler {
	return &AdminHandler{
		admin:     admin,
		detail:    detail,
		danmu:     danmu,
		reports:   reports,
		partition: partition,
		progress:  progress,
		vectors:   vectors,
		logger:    loggerOrNop(logger),
	}
}

func (h *AdminHandler) log() *zap.Logger {
	if h == nil {
		return zap.NewNop()
	}

	return loggerOrNop(h.logger)
}
