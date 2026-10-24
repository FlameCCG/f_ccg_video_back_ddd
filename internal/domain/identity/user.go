// Package identity 表达用户身份、凭据、封禁、等级与硬币规则。
//
// 从 user.go 阅读用户模型，再查 credentials.go、ban.go、exp.go、coin.go 的规则。
// 时钟、账号记录和外部身份事实由调用方提供；领域不自行读取配置、发邮件、签 JWT 或访问 OAuth。
// repository*.go 由业务需要定义接口，数据库字段、行锁和错误翻译在持久化实现中解释。
//
// 本包保持纯 Go 规则与端口，不依赖 HTTP、数据库、缓存、消息框架或配置加载器。
// 领域函数尽量显式接收事实与时间，让规则可以脱离运行环境阅读和验证。
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
