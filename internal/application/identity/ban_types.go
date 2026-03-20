package identity

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// AdminBanRecordListParams 管理端封禁记录查询参数.
type AdminBanRecordListParams struct {
	UserID   uint
	Username string
	Status   enum.UserStatus
	Page     int
	PageSize int
}

// AdminBanRecordItem 管理端封禁记录列表项（读模型）.
type AdminBanRecordItem struct {
	ID               uint            `json:"id"`
	UserID           uint            `json:"userId"`
	Username         string          `json:"username"`
	Status           enum.UserStatus `json:"status"`
	Reason           string          `json:"reason"`
	Days             int             `json:"days"`
	StartAt          time.Time       `json:"startAt"`
	EndAt            *time.Time      `json:"endAt"`
	OperatorID       uint            `json:"operatorId"`
	OperatorUsername string          `json:"operatorUsername"`
	IsUnbanned       bool            `json:"isUnbanned"`
	CreatedAt        time.Time       `json:"createdAt"`
}
