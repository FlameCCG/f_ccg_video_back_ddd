package video

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"time"
)

// ReportHandling 是一次已校验的处理意图，包含允许的原状态与必须一起保存的关联字段。
// 批量仓储以原状态作条件更新，不需要逐行读取或覆盖实体其它字段。
type ReportHandling struct {
	from, to  enum.ReportStatus
	note      string
	handlerID uint
	at        time.Time
}

func (VideoReport) PlanHandling(status enum.ReportStatus, note string, handlerID uint, at time.Time) (ReportHandling, error) {
	note, err := (VideoReport{}).ValidateHandle(status, note)
	if err != nil {
		return ReportHandling{}, err
	}
	return ReportHandling{from: enum.ReportStatusPending, to: status, note: note, handlerID: handlerID, at: at}, nil
}

func (DanmuReport) PlanHandling(status enum.ReportStatus, note string, handlerID uint, at time.Time) (ReportHandling, error) {
	return (VideoReport{}).PlanHandling(status, note, handlerID, at)
}

func (h ReportHandling) Validate() error {
	if h.from != enum.ReportStatusPending {
		return ErrReportStatusInvalid
	}
	_, err := (VideoReport{}).ValidateHandle(h.to, h.note)
	return err
}
func (h ReportHandling) FromStatus() enum.ReportStatus { return h.from }
func (h ReportHandling) Status() enum.ReportStatus     { return h.to }
func (h ReportHandling) Note() string                  { return h.note }
func (h ReportHandling) HandlerID() uint               { return h.handlerID }
func (h ReportHandling) HandledAt() time.Time          { return h.at }

func (r *VideoReport) Handle(change ReportHandling) (bool, error) {
	if err := change.Validate(); err != nil {
		return false, err
	}
	if r == nil || r.status != change.from {
		return false, nil
	}
	r.status, r.handleNote, r.handledBy = change.to, change.note, change.handlerID
	at := change.at
	r.handledAt = &at
	return true, nil
}

func (r *DanmuReport) Handle(change ReportHandling) (bool, error) {
	if err := change.Validate(); err != nil {
		return false, err
	}
	if r == nil || r.status != change.from {
		return false, nil
	}
	r.status, r.handleNote, r.handledBy = change.to, change.note, change.handlerID
	at := change.at
	r.handledAt = &at
	return true, nil
}
