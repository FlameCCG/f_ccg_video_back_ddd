package video

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	videodomain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/pkg/ctxutil"

	"go.uber.org/zap"
)

// ReportService 举报用例服务。
type ReportService struct {
	videos   VideoFactsQuery
	danmus   videodomain.DanmuRepository
	reports  videodomain.ReportRepository
	query    ReportQuery
	notify   NotifySourceQuery
	notifier VideoNotifier
	tasks    *ctxutil.DetachedTaskGroup
	logger   *zap.Logger
}

// NewReportService 构造举报用例服务。
func NewReportService(
	videos VideoFactsQuery,
	danmus videodomain.DanmuRepository,
	reports videodomain.ReportRepository,
	query ReportQuery,
	notify NotifySourceQuery,
	notifier VideoNotifier,
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
) (*VideoReportDTO, error) {
	if userID == 0 {
		return nil, videodomain.ErrUserNotLogin
	}

	if videoID == 0 {
		return nil, videodomain.ErrVideoIDRequired
	}

	report, err := videodomain.NewVideoReport("", videoID, reason, detail, images)
	if err != nil {
		return nil, err
	}

	if err := s.videos.EnsurePublished(ctx, videoID); err != nil {
		if isRecordNotFound(err) {
			return nil, videodomain.ErrVideoNotPublished
		}

		return nil, err
	}

	reporter, ok := s.notify.ActionUser(ctx, userID)
	if !ok {
		return nil, videodomain.ErrUserNotFound
	}

	report, err = videodomain.NewVideoReport(reporter.Username, videoID, reason, detail, images)
	if err != nil {
		return nil, err
	}
	report, err = s.reports.CreateVideoReport(ctx, report)
	if err != nil {
		return nil, err
	}

	return &VideoReportDTO{
		ID:               report.ID(),
		ReporterUsername: report.ReporterUsername(),
		VideoID:          report.VideoID(),
		Reason:           report.Reason(),
		Detail:           report.Detail(),
		ImageUrls:        report.ImageURLs(),
		Status:           report.Status(),
		HandleNote:       report.HandleNote(),
		HandledBy:        report.HandledBy(),
		HandledAt:        report.HandledAt(),
	}, nil
}

// CreateDanmuReport 举报弹幕。
func (s *ReportService) CreateDanmuReport(
	ctx context.Context,
	userID, danmuID uint,
	reason, detail string,
) (*DanmuReportDTO, error) {
	if userID == 0 {
		return nil, videodomain.ErrUserNotLogin
	}

	if danmuID == 0 {
		return nil, videodomain.ErrDanmuIDRequired
	}

	reason, detail, err := (videodomain.DanmuReport{}).NormalizeInput(reason, detail)
	if err != nil {
		return nil, err
	}

	danmu, err := s.danmus.FindByID(ctx, danmuID)
	if err != nil {
		return nil, err
	}

	if err := s.videos.EnsurePublished(ctx, danmu.VideoID()); err != nil {
		if isRecordNotFound(err) {
			return nil, videodomain.ErrVideoNotPublished
		}

		return nil, err
	}

	reporter, ok := s.notify.ActionUser(ctx, userID)
	if !ok {
		return nil, videodomain.ErrUserNotFound
	}

	report, err := videodomain.NewDanmuReport(reporter.Username, danmu.VideoID(), danmu.ID(), reason, detail)
	if err != nil {
		return nil, err
	}
	report, err = s.reports.CreateDanmuReport(ctx, report)
	if err != nil {
		return nil, err
	}

	return &DanmuReportDTO{
		ID:               report.ID(),
		ReporterUsername: report.ReporterUsername(),
		VideoID:          report.VideoID(),
		DanmuID:          report.DanmuID(),
		Reason:           report.Reason(),
		Detail:           report.Detail(),
		Status:           report.Status(),
		HandleNote:       report.HandleNote(),
		HandledBy:        report.HandledBy(),
		HandledAt:        report.HandledAt(),
	}, nil
}

// AdminVideoReports 管理端视频举报列表。
func (s *ReportService) AdminVideoReports(
	ctx context.Context,
	page, pageSize, status int,
) ([]AdminVideoReportItem, int64, error) {
	page, pageSize = (videodomain.ListingPolicy{}).AdminPage(page, pageSize)

	return s.query.AdminVideoReports(ctx, page, pageSize, status)
}

// AdminDanmuReports 管理端弹幕举报列表。
func (s *ReportService) AdminDanmuReports(
	ctx context.Context,
	page, pageSize, status int,
) ([]AdminDanmuReportItem, int64, error) {
	page, pageSize = (videodomain.ListingPolicy{}).AdminPage(page, pageSize)

	return s.query.AdminDanmuReports(ctx, page, pageSize, status)
}

// ValidateReportListStatus 校验举报列表筛选状态（0 表示不筛选）。
func (s *ReportService) ValidateReportListStatus(status enum.ReportStatus) error {
	return (videodomain.VideoReport{}).ValidateListStatus(status)
}

// HandleReports 管理员批量处理举报记录，并异步通知举报人。
func (s *ReportService) HandleReports(
	ctx context.Context,
	handleType videodomain.ReportHandleType,
	reportIDs []uint,
	status enum.ReportStatus,
	handleNote string,
	handlerID uint,
) (int64, error) {
	if len(reportIDs) == 0 {
		return 0, videodomain.ErrReportIDsRequired
	}

	handling, err := (videodomain.VideoReport{}).PlanHandling(status, handleNote, handlerID, time.Now())
	if err != nil {
		return 0, err
	}

	var resultRows int64

	switch handleType {
	case videodomain.ReportHandleTypeVideo:
		resultRows, err = s.reports.HandleVideoReports(ctx, reportIDs, handling)
	case videodomain.ReportHandleTypeDanmu:
		resultRows, err = s.reports.HandleDanmuReports(ctx, reportIDs, handling)
	default:
		return 0, videodomain.ErrReportTypeInvalid
	}

	if err != nil {
		return 0, err
	}

	if resultRows > 0 {
		s.tasks.Go(ctx, ctxutil.AsyncDefaultTimeout, func(asyncCtx context.Context) {
			s.notifyReportHandled(asyncCtx, handleType, reportIDs, handling.Status(), handling.Note(), handling.HandlerID())
		})
	}

	return resultRows, nil
}

// notifyReportHandled 向举报人批量发送处理结果系统通知。
func (s *ReportService) notifyReportHandled(
	ctx context.Context,
	handleType videodomain.ReportHandleType,
	reportIDs []uint,
	status enum.ReportStatus,
	handleNote string,
	handlerID uint,
) {
	if len(reportIDs) == 0 {
		return
	}

	text, ok := (videodomain.VideoReport{}).HandledNotification(handleType, status, handleNote)
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
