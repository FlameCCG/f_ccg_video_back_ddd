package entity

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// User 用户实体，是身份上下文的聚合根。
//
// JSON 字段定义接口响应；gorm tag 与关联等持久化信息由 infrastructure 的 PO 承载。
type User struct {
	RefreshTokenHash   string          `json:"-"`
	MustChangePassword bool            `json:"mustChangePassword,omitempty"`
	ID                 uint            `json:"id"`
	CreatedAt          time.Time       `json:"-"`
	UpdatedAt          time.Time       `json:"-"`
	Username           string          `json:"username"`
	Email              string          `json:"email"`
	Password           string          `json:"-"`
	Avatar             string          `json:"avatar"`
	Description        string          `json:"description"`
	Gender             int             `json:"gender"`
	Birthday           *time.Time      `json:"birthday"`
	Level              int             `json:"level"`
	OpenID             string          `json:"openID"`
	RegisterSource     string          `json:"registerSource"`
	Exp                int64           `json:"exp"`
	CoinCount          float64         `json:"coinCount"`
	Status             enum.UserStatus `json:"status"`
	CurrentRoleID      uint            `json:"currentRoleID"`
	RegisterIP         string          `json:"registerIP"`
	LastLoginIP        string          `json:"lastLoginIP"`
	RegisterAddr       string          `json:"registerAddr"`
	LastLoginAddr      string          `json:"lastLoginAddr"`
}

// IsBanned 判断用户当前是否处于封禁态（限时或永久）。
func (u *User) IsBanned() bool {
	if u == nil {
		return false
	}

	return u.Status == enum.UserStatusBanned || u.Status == enum.UserStatusPermanentlyBanned
}

// IsPermanentlyBanned 判断用户是否被永久封禁。
func (u *User) IsPermanentlyBanned() bool {
	if u == nil {
		return false
	}

	return u.Status == enum.UserStatusPermanentlyBanned
}
