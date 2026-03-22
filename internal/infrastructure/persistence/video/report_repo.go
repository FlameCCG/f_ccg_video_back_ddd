package video

import (
	"context"
	"encoding/json"

	videodomain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ReportRepository 举报写侧仓储的 GORM 实现。
type ReportRepository struct {
	db *gorm.DB
}

var _ videodomain.ReportRepository = (*ReportRepository)(nil)

// NewReportRepository 构造举报仓储。
func NewReportRepository(db *gorm.DB) *ReportRepository {
	return &ReportRepository{db: db}
}

// CreateVideoReport 写入视频举报并回填 ID/时间。
func (r *ReportRepository) CreateVideoReport(ctx context.Context, report *videodomain.VideoReport) (*videodomain.VideoReport, error) {
	if report == nil {
		return nil, videodomain.ErrParamInvalid
	}

	row := po.VideoReport{
		ReporterUsername: report.ReporterUsername(),
		VideoID:          report.VideoID(),
		Reason:           report.Reason(),
		Detail:           report.Detail(),
		ImageUrls:        marshalImageJSON(report.ImageURLs()),
		Status:           report.Status(),
		HandleNote:       report.HandleNote(),
		HandledBy:        report.HandledBy(),
		HandledAt:        report.HandledAt(),
	}

	if err := gormx.DB(ctx, r.db).Create(&row).Error; err != nil {
		return nil, err
	}

	snap := report.Snapshot()
	snap.ID = row.ID
	snap.CreatedAt = row.CreatedAt
	snap.UpdatedAt = row.UpdatedAt

	return videodomain.RestoreVideoReport(snap), nil
}

// CreateDanmuReport 写入弹幕举报并回填 ID/时间。
func (r *ReportRepository) CreateDanmuReport(ctx context.Context, report *videodomain.DanmuReport) (*videodomain.DanmuReport, error) {
	if report == nil {
		return nil, videodomain.ErrParamInvalid
	}

	row := po.DanmuReport{
		ReporterUsername: report.ReporterUsername(),
		VideoID:          report.VideoID(),
		DanmuID:          report.DanmuID(),
		Reason:           report.Reason(),
		Detail:           report.Detail(),
		Status:           report.Status(),
		HandleNote:       report.HandleNote(),
		HandledBy:        report.HandledBy(),
		HandledAt:        report.HandledAt(),
	}

	if err := gormx.DB(ctx, r.db).Create(&row).Error; err != nil {
		return nil, err
	}

	snap := report.Snapshot()
	snap.ID = row.ID
	snap.CreatedAt = row.CreatedAt
	snap.UpdatedAt = row.UpdatedAt

	return videodomain.RestoreDanmuReport(snap), nil
}

// HandleVideoReports 批量处理待处理的视频举报，返回影响行数。
func (r *ReportRepository) HandleVideoReports(
	ctx context.Context,
	reportIDs []uint,
	handling videodomain.ReportHandling,
) (int64, error) {
	if err := handling.Validate(); err != nil {
		return 0, err
	}
	result := gormx.DB(ctx, r.db).
		Model(&po.VideoReport{}).
		Where("id IN ? AND status = ?", reportIDs, handling.FromStatus()).
		Updates(reportHandleUpdates(handling))

	return result.RowsAffected, result.Error
}

// HandleDanmuReports 批量处理待处理的弹幕举报，返回影响行数。
func (r *ReportRepository) HandleDanmuReports(
	ctx context.Context,
	reportIDs []uint,
	handling videodomain.ReportHandling,
) (int64, error) {
	if err := handling.Validate(); err != nil {
		return 0, err
	}
	result := gormx.DB(ctx, r.db).
		Model(&po.DanmuReport{}).
		Where("id IN ? AND status = ?", reportIDs, handling.FromStatus()).
		Updates(reportHandleUpdates(handling))

	return result.RowsAffected, result.Error
}

// ReporterUsernames 读取给定举报记录的举报人用户名列表。
func (r *ReportRepository) ReporterUsernames(
	ctx context.Context,
	handleType videodomain.ReportHandleType,
	reportIDs []uint,
) ([]string, error) {
	if len(reportIDs) == 0 {
		return nil, nil
	}

	var table string

	switch handleType {
	case videodomain.ReportHandleTypeVideo:
		table = "video_reports"
	case videodomain.ReportHandleTypeDanmu:
		table = "danmu_reports"
	default:
		return nil, videodomain.ErrReportTypeInvalid
	}

	type row struct {
		ReporterUsername string `gorm:"column:reporter_username"`
	}

	var rows []row
	if err := gormx.DB(ctx, r.db).
		Table(table).
		Select("reporter_username").
		Where("id IN ?", reportIDs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	usernames := make([]string, 0, len(rows))
	for _, item := range rows {
		usernames = append(usernames, item.ReporterUsername)
	}

	return usernames, nil
}

// reportHandleUpdates 组装举报处理的列更新。
func reportHandleUpdates(handling videodomain.ReportHandling) map[string]any {
	at := handling.HandledAt()
	return map[string]any{"status": handling.Status(), "handle_note": handling.Note(), "handled_by": handling.HandlerID(), "handled_at": &at}
}

// marshalImageJSON 把图片 URL 列表序列化为 JSON 列值；空列表时返回零值（NULL）。
func marshalImageJSON(images []string) datatypes.JSON {
	if len(images) == 0 {
		return nil
	}

	data, err := json.Marshal(images)
	if err != nil {
		return nil
	}

	return datatypes.JSON(data)
}

// parseImageJSON 解析图片 URL 列表 JSON 列值；失败时返回 nil。
func parseImageJSON(raw datatypes.JSON) []string {
	if len(raw) == 0 {
		return nil
	}

	var images []string
	if err := json.Unmarshal(raw, &images); err != nil {
		return nil
	}

	return images
}
