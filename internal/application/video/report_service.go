package video

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

// ReportService 举报用例服务
// （原 service/video_service/{report_service.go,report_admin_service.go,
// report_handle_service.go}）。
type ReportService struct {
	videos   domain.Repository
	danmus   domain.DanmuRepository
	reports  domain.ReportRepository
	query    ReportQuery
	notify   NotifySourceQuery
	notifier Notifier
	tasks    *ctxutil.DetachedTaskGroup
	logger   *zap.Logger
}

// NewReportService 构造举报用例服务。
func NewReportService(
	videos domain.Repository,
	danmus domain.DanmuRepository,
	reports domain.ReportRepository,
	query ReportQuery,
	notify NotifySourceQuery,
	notifier Notifier,
	tasks *ctxutil.DetachedTaskGroup,
	logger *zap.Logger,
) *ReportService {
	return &ReportService{
		videos:   videos,
		danmus:   danmus,
		reports:  reports,
		query:    query,
		notify:   notify,
		notifier: notifier,
		tasks:    tasks,
		logger:   logger,
	}
}

// CreateVideoReport 举报视频。
func (s *ReportService) CreateVideoReport(
	ctx context.Context,
	userID, videoID uint,
	reason, detail string,
	images []string,
) (*ReportDTO, error) {
	if userID == 0 {
		return nil, domain.ErrUserNotLogin
	}

	if videoID == 0 {
		return nil, domain.ErrVideoIDRequired
	}

	reason, detail, err := domain.NormalizeReportInput(reason, detail)
	if err != nil {
		return nil, err
	}

	if err := s.videos.EnsurePublished(ctx, videoID); err != nil {
		if isRecordNotFound(err) {
			return nil, domain.ErrVideoNotPublished
		}

		return nil, err
	}

	reporter, ok := s.notify.ActionUser(ctx, userID)
	if !ok {
		return nil, domain.ErrUserNotFound
	}

	report := &domain.Report{
		ReporterUsername: reporter.Username,
		VideoID:          videoID,
		Reason:           reason,
		Detail:           detail,
		ImageURLs:        images,
		Status:           enum.ReportStatusPending,
	}
	if err := s.reports.CreateVideoReport(ctx, report); err != nil {
		return nil, err
	}

	return &ReportDTO{
		ID:               report.ID,
		ReporterUsername: report.ReporterUsername,
		VideoID:          report.VideoID,
		Reason:           report.Reason,
		Detail:           report.Detail,
		ImageUrls:        report.ImageURLs,
		Status:           report.Status,
		HandleNote:       report.HandleNote,
		HandledBy:        report.HandledBy,
		HandledAt:        report.HandledAt,
	}, nil
}

// CreateDanmuReport 举报弹幕。
func (s *ReportService) CreateDanmuReport(
	ctx context.Context,
	userID, danmuID uint,
	reason, detail string,
) (*DanmuReportDTO, error) {
	if userID == 0 {
		return nil, domain.ErrUserNotLogin
	}

	if danmuID == 0 {
		return nil, domain.ErrDanmuIDRequired
	}

	reason, detail, err := domain.NormalizeReportInput(reason, detail)
	if err != nil {
		return nil, err
	}

	danmu, err := s.danmus.FindByID(ctx, danmuID)
	if err != nil {
		return nil, err
	}

	if err := s.videos.EnsurePublished(ctx, danmu.VideoID); err != nil {
		if isRecordNotFound(err) {
			return nil, domain.ErrVideoNotPublished
		}

		return nil, err
	}

	reporter, ok := s.notify.ActionUser(ctx, userID)
	if !ok {
		return nil, domain.ErrUserNotFound
	}

	report := &domain.DanmuReport{
		ReporterUsername: reporter.Username,
		VideoID:          danmu.VideoID,
		DanmuID:          danmu.ID,
		Reason:           reason,
		Detail:           detail,
		Status:           enum.ReportStatusPending,
	}
	if err := s.reports.CreateDanmuReport(ctx, report); err != nil {
		return nil, err
	}

	return &DanmuReportDTO{
		ID:               report.ID,
		ReporterUsername: report.ReporterUsername,
		VideoID:          report.VideoID,
		DanmuID:          report.DanmuID,
		Reason:           report.Reason,
		Detail:           report.Detail,
		Status:           report.Status,
		HandleNote:       report.HandleNote,
		HandledBy:        report.HandledBy,
		HandledAt:        report.HandledAt,
	}, nil
}

// AdminVideoReports 管理端视频举报列表。
func (s *ReportService) AdminVideoReports(
	ctx context.Context,
	page, pageSize, status int,
) ([]AdminVideoReportItem, int64, error) {
	page, pageSize = normalizeReportPage(page, pageSize)

	return s.query.AdminVideoReports(ctx, page, pageSize, status)
}

// AdminDanmuReports 管理端弹幕举报列表。
func (s *ReportService) AdminDanmuReports(
	ctx context.Context,
	page, pageSize, status int,
) ([]AdminDanmuReportItem, int64, error) {
	page, pageSize = normalizeReportPage(page, pageSize)

	return s.query.AdminDanmuReports(ctx, page, pageSize, status)
}

// ValidateReportListStatus 校验举报列表筛选状态（0 表示不筛选）。
func (s *ReportService) ValidateReportListStatus(status enum.ReportStatus) error {
	return domain.ValidateReportListStatus(status)
}

// HandleReports 管理员批量处理举报记录，并异步通知举报人。
func (s *ReportService) HandleReports(
	ctx context.Context,
	handleType domain.ReportHandleType,
	reportIDs []uint,
	status enum.ReportStatus,
	handleNote string,
	handlerID uint,
) (int64, error) {
	if len(reportIDs) == 0 {
		return 0, domain.ErrReportIDsRequired
	}

	note, err := domain.ValidateReportHandle(status, handleNote)
	if err != nil {
		return 0, err
	}

	now := time.Now()

	var resultRows int64

	switch handleType {
	case domain.ReportHandleTypeVideo:
		resultRows, err = s.reports.HandleVideoReports(ctx, reportIDs, status, note, handlerID, now)
	case domain.ReportHandleTypeDanmu:
		resultRows, err = s.reports.HandleDanmuReports(ctx, reportIDs, status, note, handlerID, now)
	default:
		return 0, domain.ErrReportTypeInvalid
	}

	if err != nil {
		return 0, err
	}

	if resultRows > 0 {
		s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
			s.notifyReportHandled(asyncCtx, handleType, reportIDs, status, note, handlerID)
		})
	}

	return resultRows, nil
}

// notifyReportHandled 向举报人批量发送处理结果系统通知。
func (s *ReportService) notifyReportHandled(
	ctx context.Context,
	handleType domain.ReportHandleType,
	reportIDs []uint,
	status enum.ReportStatus,
	handleNote string,
	handlerID uint,
) {
	if len(reportIDs) == 0 {
		return
	}

	text, ok := domain.BuildReportHandledNotification(handleType, status, handleNote)
	if !ok {
		return
	}

	usernames, err := s.reports.ReporterUsernames(ctx, handleType, reportIDs)
	if err != nil {
		s.logger.Warn("load report reporters failed", zap.Error(err), zap.String("type", string(handleType)))

		return
	}

	if len(usernames) == 0 {
		return
	}

	usernameSet := make(map[string]struct{}, len(usernames))
	for _, name := range usernames {
		if name == "" {
			continue
		}

		usernameSet[name] = struct{}{}
	}

	if len(usernameSet) == 0 {
		return
	}

	unique := make([]string, 0, len(usernameSet))
	for name := range usernameSet {
		unique = append(unique, name)
	}

	userIDMap, err := s.notify.UserIDsByUsernames(ctx, unique)
	if err != nil {
		s.logger.Warn("load report users failed", zap.Error(err))

		return
	}

	if len(userIDMap) == 0 {
		return
	}

	var (
		actionUserID     uint
		actionUserName   string
		actionUserAvatar string
	)

	if handlerID > 0 {
		if handler, found := s.notify.ActionUser(ctx, handlerID); found {
			actionUserID = handler.ID
			actionUserName = handler.Username
			actionUserAvatar = handler.Avatar
		} else {
			actionUserID = handlerID
		}
	}

	notices := make([]SystemNotice, 0, len(userIDMap))

	for name, userID := range userIDMap {
		if userID == 0 || name == "" {
			continue
		}

		notices = append(notices, SystemNotice{
			ReceiverID:       userID,
			ActionUserID:     actionUserID,
			ActionUserName:   actionUserName,
			ActionUserAvatar: actionUserAvatar,
			Title:            text.Title,
			Content:          text.Content,
		})
	}

	if len(notices) == 0 {
		return
	}

	s.notifier.NotifySystemBatch(ctx, notices)
}

func normalizeReportPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	return page, pageSize
}
