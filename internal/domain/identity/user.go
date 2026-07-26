// Package identity 是身份上下文的领域层：用户实体、等级/硬币/封禁规则与仓储接口。
//
// 本包为纯 Go：不依赖 gin / gorm / redis / viper / casbin 等任何框架。
package identity

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// User 用户实体（身份上下文的聚合根）。
//
// 字段与 json tag 与原 models/mysqlEntity.User 对外序列化部分保持一致，
// 保证接口响应 JSON 1:1；持久化细节（gorm tag、关联）留在 infrastructure 的 PO。
type User struct {
	ID             uint            `json:"id"`
	CreatedAt      time.Time       `json:"-"`
	UpdatedAt      time.Time       `json:"-"`
	Username       string          `json:"username"`
	Email          string          `json:"email"`
	Password       string          `json:"-"`
	Avatar         string          `json:"avatar"`
	Description    string          `json:"description"`
	Gender         int             `json:"gender"`
	Birthday       *time.Time      `json:"birthday"`
	Level          int             `json:"level"`
	OpenID         string          `json:"openID"`
	RegisterSource string          `json:"registerSource"`
	Exp            int64           `json:"exp"`
	CoinCount      float64         `json:"coinCount"`
	Status         enum.UserStatus `json:"status"`
	CurrentRoleID  uint            `json:"currentRoleID"`
	RegisterIP     string          `json:"registerIP"`
	LastLoginIP    string          `json:"lastLoginIP"`
	RegisterAddr   string          `json:"registerAddr"`
	LastLoginAddr  string          `json:"lastLoginAddr"`
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
