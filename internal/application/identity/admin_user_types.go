package identity

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// AdminUserListParams 管理端用户列表查询参数.
type AdminUserListParams struct {
	Keyword  string
	Status   enum.UserStatus
	RoleID   uint
	Page     int
	PageSize int
}

// AdminUserListItem 管理端用户列表项（读模型）.
type AdminUserListItem struct {
	ID             uint            `json:"id"`
	Username       string          `json:"username"`
	Email          string          `json:"email"`
	Avatar         string          `json:"avatar"`
	Status         enum.UserStatus `json:"status"`
	Level          int             `json:"level"`
	Exp            int64           `json:"exp"`
	CoinCount      float64         `json:"coinCount"`
	CurrentRoleID  uint            `json:"currentRoleId"`
	RegisterSource string          `json:"registerSource"`
	RoleNames      []string        `json:"roleNames"`
	CreatedAt      time.Time       `json:"createdAt"`
}
