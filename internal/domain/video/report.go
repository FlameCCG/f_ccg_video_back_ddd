package video

import (
	"slices"
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// maxReportHandleNoteLen 处理说明最大长度。
const maxReportHandleNoteLen = 500

// ReportHandleType 举报处理类型。
type ReportHandleType string

// 举报处理类型枚举。
const (
	// ReportHandleTypeVideo 视频举报处理.
	ReportHandleTypeVideo ReportHandleType = "video"
	// ReportHandleTypeDanmu 弹幕举报处理.
	ReportHandleTypeDanmu ReportHandleType = "danmu"
)

// VideoReport 视频举报实体。
type VideoReport struct {
	id               uint
	createdAt        time.Time
	updatedAt        time.Time
	reporterUsername string
	videoID          uint
	reason           string
	detail           string
	imageURLs        []string
	status           enum.ReportStatus
	handleNote       string
	handledBy        uint
	handledAt        *time.Time
}

// VideoReportSnapshot 是视频举报的持久化快照。
type VideoReportSnapshot struct {
	ID               uint
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ReporterUsername string
	VideoID          uint
	Reason           string
	Detail           string
	ImageURLs        []string
	Status           enum.ReportStatus
	HandleNote       string
	HandledBy        uint
	HandledAt        *time.Time
}

// NewVideoReport 校验原因后创建待处理的视频举报。图片切片会被复制。
func NewVideoReport(reporter string, videoID uint, reason, detail string, images []string) (*VideoReport, error) {
	reason, detail, err := (VideoReport{}).NormalizeInput(reason, detail)
	if err != nil {
		return nil, err
	}

	return &VideoReport{
		reporterUsername: reporter,
		videoID:          videoID,
		reason:           reason,
		detail:           detail,
		imageURLs:        slices.Clone(images),
		status:           enum.ReportStatusPending,
	}, nil
}

// RestoreVideoReport 从快照恢复视频举报，不重新校验原因。
func RestoreVideoReport(snapshot VideoReportSnapshot) *VideoReport {
	return &VideoReport{
		id:               snapshot.ID,
		createdAt:        snapshot.CreatedAt,
		updatedAt:        snapshot.UpdatedAt,
		reporterUsername: snapshot.ReporterUsername,
		videoID:          snapshot.VideoID,
		reason:           snapshot.Reason,
		detail:           snapshot.Detail,
		imageURLs:        slices.Clone(snapshot.ImageURLs),
		status:           snapshot.Status,
		handleNote:       snapshot.HandleNote,
		handledBy:        snapshot.HandledBy,
		handledAt:        (Video{}).cloneTime(snapshot.HandledAt),
	}
}

// Snapshot 返回视频举报快照。图片与处理时间都是副本。
func (r *VideoReport) Snapshot() VideoReportSnapshot {
	if r == nil {
		return VideoReportSnapshot{}
	}

	return VideoReportSnapshot{
		ID:               r.id,
		CreatedAt:        r.createdAt,
		UpdatedAt:        r.updatedAt,
		ReporterUsername: r.reporterUsername,
		VideoID:          r.videoID,
		Reason:           r.reason,
		Detail:           r.detail,
		ImageURLs:        slices.Clone(r.imageURLs),
		Status:           r.status,
		HandleNote:       r.handleNote,
		HandledBy:        r.handledBy,
		HandledAt:        (Video{}).cloneTime(r.handledAt),
	}
}

func (r *VideoReport) ID() uint {
	if r == nil {
		return 0
	}

	return r.id
}

func (r *VideoReport) CreatedAt() time.Time {
	if r == nil {
		return time.Time{}
	}

	return r.createdAt
}

func (r *VideoReport) UpdatedAt() time.Time {
	if r == nil {
		return time.Time{}
	}

	return r.updatedAt
}

func (r *VideoReport) ReporterUsername() string {
	if r == nil {
		return ""
	}

	return r.reporterUsername
}

func (r *VideoReport) VideoID() uint {
	if r == nil {
		return 0
	}

	return r.videoID
}

func (r *VideoReport) Reason() string {
	if r == nil {
		return ""
	}

	return r.reason
}

func (r *VideoReport) Detail() string {
	if r == nil {
		return ""
	}

	return r.detail
}

// ImageURLs 返回图片地址副本。
func (r *VideoReport) ImageURLs() []string {
	if r == nil {
		return nil
	}

	return slices.Clone(r.imageURLs)
}

func (r *VideoReport) Status() enum.ReportStatus {
	if r == nil {
		return 0
	}

	return r.status
}

func (r *VideoReport) HandleNote() string {
	if r == nil {
		return ""
	}

	return r.handleNote
}

func (r *VideoReport) HandledBy() uint {
	if r == nil {
		return 0
	}

	return r.handledBy
}

// HandledAt 返回处理时间副本。
func (r *VideoReport) HandledAt() *time.Time {
	if r == nil {
		return nil
	}

	return (Video{}).cloneTime(r.handledAt)
}

// DanmuReport 弹幕举报实体。
type DanmuReport struct {
	id               uint
	createdAt        time.Time
	updatedAt        time.Time
	reporterUsername string
	videoID          uint
	danmuID          uint
	reason           string
	detail           string
	status           enum.ReportStatus
	handleNote       string
	handledBy        uint
	handledAt        *time.Time
}

// DanmuReportSnapshot 是弹幕举报的持久化快照。
type DanmuReportSnapshot struct {
	ID               uint
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ReporterUsername string
	VideoID          uint
	DanmuID          uint
	Reason           string
	Detail           string
	Status           enum.ReportStatus
	HandleNote       string
	HandledBy        uint
	HandledAt        *time.Time
}

// NewDanmuReport 校验原因后创建待处理的弹幕举报。
func NewDanmuReport(reporter string, videoID, danmuID uint, reason, detail string) (*DanmuReport, error) {
	reason, detail, err := (DanmuReport{}).NormalizeInput(reason, detail)
	if err != nil {
		return nil, err
	}

	return &DanmuReport{
		reporterUsername: reporter,
		videoID:          videoID,
		danmuID:          danmuID,
		reason:           reason,
		detail:           detail,
		status:           enum.ReportStatusPending,
	}, nil
}

// RestoreDanmuReport 从快照恢复弹幕举报，不重新校验原因。
func RestoreDanmuReport(snapshot DanmuReportSnapshot) *DanmuReport {
	return &DanmuReport{
		id:               snapshot.ID,
		createdAt:        snapshot.CreatedAt,
		updatedAt:        snapshot.UpdatedAt,
		reporterUsername: snapshot.ReporterUsername,
		videoID:          snapshot.VideoID,
		danmuID:          snapshot.DanmuID,
		reason:           snapshot.Reason,
		detail:           snapshot.Detail,
		status:           snapshot.Status,
		handleNote:       snapshot.HandleNote,
		handledBy:        snapshot.HandledBy,
		handledAt:        (Video{}).cloneTime(snapshot.HandledAt),
	}
}

// Snapshot 返回弹幕举报快照。
func (r *DanmuReport) Snapshot() DanmuReportSnapshot {
	if r == nil {
		return DanmuReportSnapshot{}
	}

	return DanmuReportSnapshot{
		ID:               r.id,
		CreatedAt:        r.createdAt,
		UpdatedAt:        r.updatedAt,
		ReporterUsername: r.reporterUsername,
		VideoID:          r.videoID,
		DanmuID:          r.danmuID,
		Reason:           r.reason,
		Detail:           r.detail,
		Status:           r.status,
		HandleNote:       r.handleNote,
		HandledBy:        r.handledBy,
		HandledAt:        (Video{}).cloneTime(r.handledAt),
	}
}

func (r *DanmuReport) ID() uint {
	if r == nil {
		return 0
	}

	return r.id
}

func (r *DanmuReport) CreatedAt() time.Time {
	if r == nil {
		return time.Time{}
	}

	return r.createdAt
}

func (r *DanmuReport) UpdatedAt() time.Time {
	if r == nil {
		return time.Time{}
	}

	return r.updatedAt
}

func (r *DanmuReport) ReporterUsername() string {
	if r == nil {
		return ""
	}

	return r.reporterUsername
}

func (r *DanmuReport) VideoID() uint {
	if r == nil {
		return 0
	}

	return r.videoID
}

func (r *DanmuReport) DanmuID() uint {
	if r == nil {
		return 0
	}

	return r.danmuID
}

func (r *DanmuReport) Reason() string {
	if r == nil {
		return ""
	}

	return r.reason
}

func (r *DanmuReport) Detail() string {
	if r == nil {
		return ""
	}

	return r.detail
}

func (r *DanmuReport) Status() enum.ReportStatus {
	if r == nil {
		return 0
	}

	return r.status
}

func (r *DanmuReport) HandleNote() string {
	if r == nil {
		return ""
	}

	return r.handleNote
}

func (r *DanmuReport) HandledBy() uint {
	if r == nil {
		return 0
	}

	return r.handledBy
}

// HandledAt 返回处理时间副本。
func (r *DanmuReport) HandledAt() *time.Time {
	if r == nil {
		return nil
	}

	return (Video{}).cloneTime(r.handledAt)
}

// NormalizeInput 校验并归一化举报原因与详情。
func (VideoReport) NormalizeInput(reason, detail string) (string, string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", "", ErrReportReasonEmpty
	}

	return reason, strings.TrimSpace(detail), nil
}

// NormalizeInput 校验并归一化弹幕举报原因与详情。
func (DanmuReport) NormalizeInput(reason, detail string) (string, string, error) {
	return (VideoReport{}).NormalizeInput(reason, detail)
}

// ValidateHandle 校验举报处理入参（目标状态与处理说明）。
func (VideoReport) ValidateHandle(status enum.ReportStatus, handleNote string) (string, error) {
	switch status {
	case enum.ReportStatusResolved, enum.ReportStatusRejected:
	default:
		return "", ErrReportStatusInvalid
	}

	note := strings.TrimSpace(handleNote)
	if len(note) > maxReportHandleNoteLen {
		return "", ErrReportNoteTooLong
	}

	return note, nil
}

// ValidateHandle 校验弹幕举报处理入参。
func (DanmuReport) ValidateHandle(status enum.ReportStatus, handleNote string) (string, error) {
	return (VideoReport{}).ValidateHandle(status, handleNote)
}

// ReportHandledNotification 举报处理结果通知的标题与正文。
type ReportHandledNotification struct {
	Title   string
	Content string
}

// HandledNotification 组装举报处理结果通知文案。
//
// 文案。
func (VideoReport) HandledNotification(
	handleType ReportHandleType,
	status enum.ReportStatus,
	handleNote string,
) (ReportHandledNotification, bool) {
	var title, contentPrefix string

	switch handleType {
	case ReportHandleTypeVideo:
		title = "视频举报处理结果"
		contentPrefix = "您提交的视频举报已"
	case ReportHandleTypeDanmu:
		title = "弹幕举报处理结果"
		contentPrefix = "您提交的弹幕举报已"
	default:
		return ReportHandledNotification{}, false
	}

	content := contentPrefix

	switch status {
	case enum.ReportStatusResolved:
		content += "处理。"
	case enum.ReportStatusRejected:
		content += "驳回。"
	default:
		content += "处理。"
	}

	if handleNote != "" {
		content += "处理说明：" + handleNote
	} else {
		switch status {
		case enum.ReportStatusResolved:
			content += "处理说明：感谢反馈，我们已采取相应措施。"
		case enum.ReportStatusRejected:
			content += "处理说明：经核查暂不符合处理条件。"
		default:
			content += "处理说明：感谢反馈，我们已记录。"
		}
	}

	return ReportHandledNotification{Title: title, Content: content}, true
}

// HandledNotification 组装弹幕举报处理结果通知文案。
func (DanmuReport) HandledNotification(
	handleType ReportHandleType,
	status enum.ReportStatus,
	handleNote string,
) (ReportHandledNotification, bool) {
	return (VideoReport{}).HandledNotification(handleType, status, handleNote)
}

// ValidateListStatus 校验举报列表筛选状态（0 表示不筛选）。
func (VideoReport) ValidateListStatus(status enum.ReportStatus) error {
	if status == 0 {
		return nil
	}

	switch status {
	case enum.ReportStatusPending, enum.ReportStatusResolved, enum.ReportStatusRejected:
		return nil
	default:
		return ErrReportStatusInvalid
	}
}

// ValidateListStatus 校验弹幕举报列表筛选状态。
func (DanmuReport) ValidateListStatus(status enum.ReportStatus) error {
	return (VideoReport{}).ValidateListStatus(status)
}
