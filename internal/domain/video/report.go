package video

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"strings"
	"time"
)

// maxReportHandleNoteLen 处理说明最大长度（与原 HandleReports 一致，按字节计）。
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

// Report 视频举报实体。
type Report struct {
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

// DanmuReport 弹幕举报实体。
type DanmuReport struct {
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

// NormalizeReportInput 校验并归一化举报原因与详情。
func NormalizeReportInput(reason, detail string) (string, string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", "", ErrReportReasonEmpty
	}

	return reason, strings.TrimSpace(detail), nil
}

// ValidateReportHandle 校验举报处理入参（目标状态与处理说明）。
func ValidateReportHandle(status enum.ReportStatus, handleNote string) (string, error) {
	switch status {
	case enum.ReportStatusResolved, enum.ReportStatusRejected:
	case enum.ReportStatusPending:
		fallthrough
	default:
		return "", ErrReportStatusInvalid
	}

	note := strings.TrimSpace(handleNote)
	if len(note) > maxReportHandleNoteLen {
		return "", ErrReportNoteTooLong
	}

	return note, nil
}

// ReportHandledNotification 举报处理结果通知的标题与正文。
type ReportHandledNotification struct {
	Title   string
	Content string
}

// BuildReportHandledNotification 组装举报处理结果通知文案。
//
// 文案与原 notifyReportHandled 逐字一致。
func BuildReportHandledNotification(
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
	case enum.ReportStatusPending:
		fallthrough
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
		case enum.ReportStatusPending:
			fallthrough
		default:
			content += "处理说明：感谢反馈，我们已记录。"
		}
	}

	return ReportHandledNotification{Title: title, Content: content}, true
}

// ValidateReportListStatus 校验举报列表筛选状态（0 表示不筛选）。
func ValidateReportListStatus(status enum.ReportStatus) error {
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
