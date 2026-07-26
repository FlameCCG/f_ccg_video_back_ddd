package identity

import (
	"io"
	"time"
)

// EmailRegisterInput 邮箱注册入参。
type EmailRegisterInput struct {
	Username     string
	Password     string
	Email        string
	RegisterIP   string
	RegisterAddr string
}

// PwdLoginInput 用户名/邮箱 + 密码登录入参。
type PwdLoginInput struct {
	Identifier string
	Password   string
	IP         string
	Addr       string
	UserAgent  string
}

// OAuthLoginInput 第三方登录入参。
type OAuthLoginInput struct {
	// Provider 第三方标识（qq/google/github/linuxdo/x），同时作为 register_source。
	Provider string
	// Code 授权码。
	Code string
	// CodeVerifier PKCE 校验串（github 可选、x 必填）。
	CodeVerifier string
	// IP 客户端 IP。
	IP string
	// Addr 客户端 IP 归属地。
	Addr string
	// UserAgent 请求 UA，用于登录记录。
	UserAgent string
}

// UserInfoResult 当前登录用户信息（json 字段与原 UserInfoRes 逐字一致）。
type UserInfoResult struct {
	ID             uint     `json:"id"`
	Username       string   `json:"username"`
	Email          string   `json:"email"`
	Avatar         string   `json:"avatar"`
	BannerURL      string   `json:"bannerUrl"`
	Description    string   `json:"description"`
	Gender         int      `json:"gender"`
	Birthday       string   `json:"birthday"`
	Level          int      `json:"level"`
	Exp            int64    `json:"exp"`
	CoinCount      float64  `json:"coinCount"`
	FollowCount    int64    `json:"followCount"`
	FansCount      int64    `json:"fansCount"`
	DynamicCount   int64    `json:"dynamicCount"`
	RegisterSource string   `json:"registerSource"`
	LikeTags       []string `json:"likeTags"`
	// HasPassword 是否已设置登录密码；第三方首次登录用户可能为 false。
	// 前端「修改密码」可据此决定是否要求输入旧密码。
	HasPassword bool `json:"hasPassword"`
	// IsThirdParty 是否绑定了第三方登录（OpenID 非空）。
	IsThirdParty bool `json:"isThirdParty"`
}

// UserDetailResult 用户公开详情（json 字段与原 UserDetailRes 逐字一致）。
type UserDetailResult struct {
	ID          uint     `json:"id"`
	Username    string   `json:"username"`
	Avatar      string   `json:"avatar"`
	BannerURL   string   `json:"bannerUrl"`
	Description string   `json:"description"`
	Gender      int      `json:"gender"`
	Birthday    string   `json:"birthday"`
	Level       int      `json:"level"`
	Exp         int64    `json:"exp"`
	FollowCount int64    `json:"followCount"`
	FansCount   int64    `json:"fansCount"`
	TotalLikes  int64    `json:"totalLikes"`
	TotalViews  int64    `json:"totalViews"`
	LikeTags    []string `json:"likeTags"`
}

// AdminUserInfoResult 后台当前登录用户信息（json 字段与原 AdminUserInfoRes 逐字一致）。
type AdminUserInfoResult struct {
	ID             uint            `json:"id"`
	Username       string          `json:"username"`
	Email          string          `json:"email"`
	Avatar         string          `json:"avatar"`
	Description    string          `json:"description"`
	Gender         int             `json:"gender"`
	Birthday       string          `json:"birthday"`
	Level          int             `json:"level"`
	Exp            int64           `json:"exp"`
	CoinCount      float64         `json:"coinCount"`
	FollowCount    int64           `json:"followCount"`
	FansCount      int64           `json:"fansCount"`
	DynamicCount   int64           `json:"dynamicCount"`
	CurrentRoleID  uint            `json:"currentRoleID"`
	RegisterSource string          `json:"registerSource"`
	RoleMap        map[uint]string `json:"roleMap"`
}

// UpdateProfileInput 更新用户资料入参（零值字段按原实现语义视为「不更新」）。
type UpdateProfileInput struct {
	// UserID 待更新用户；用户侧取当前登录用户，管理端取请求参数。
	UserID uint
	// Username 空串表示不更新。
	Username string
	// Avatar 空串表示不更新。
	Avatar string
	// Description 空串表示不更新。
	Description string
	// Gender nil 表示不更新。
	Gender *int
	// Birthday 空串表示不更新，否则必须是 2006-01-02。
	Birthday string
}

// UpdateUserConfInput 更新用户配置入参。
type UpdateUserConfInput struct {
	UserID        uint
	OpenCollect   *bool
	OpenFans      *bool
	OpenFollow    *bool
	OpenLikeVideo *bool
	OpenCoinVideo *bool
	BannerID      *uint
	BannerURL     string
	LikeTags      *[]string
}

// BindEmailInput 绑定邮箱入参。
type BindEmailInput struct {
	UserID uint
	Email  string
}

// ResetPasswordInput 邮箱验证码重置密码入参。
type ResetPasswordInput struct {
	Email       string
	NewPassword string
}

// ChangePasswordInput 登录态修改密码入参。
type ChangePasswordInput struct {
	UserID      uint
	OldPassword string
	NewPassword string
}

// UploadBannerInput 上传用户主页横幅入参。
type UploadBannerInput struct {
	// FileHash 前端计算的文件哈希，同时作为对象名。
	FileHash string
	// File 上传文件内容，必须可 Seek（图片嗅探需要回绕）。
	File io.ReadSeeker
	// Filename 原始文件名，用于扩展名校验。
	Filename string
	// Size 文件字节数。
	Size int64
}

// DefaultUserBannerList 系统默认用户主页横幅列表
// （json 字段与原 DefaultUserBannerListRes 逐字一致）。
type DefaultUserBannerList struct {
	// BannerIDs 是系统配置中的默认横幅ID顺序。
	BannerIDs             []uint           `json:"bannerIds"`
	CurrentBannerID       uint             `json:"currentBannerId"`
	CurrentBannerImageURL string           `json:"currentBannerImageUrl"`
	List                  []UserBannerItem `json:"list"`
}

// AdminUpdateUserBannerListResult 管理端增删系统默认横幅后的结果
// （json 字段与原 adminUpdateUserBannerListRes 逐字一致）。
type AdminUpdateUserBannerListResult struct {
	BannerIDs []uint `json:"bannerIds"`
}

// EmailCaptchaSendInput 发送邮箱验证码入参。
type EmailCaptchaSendInput struct {
	// Type 1 注册 / 2 重置密码 / 3 绑定邮箱。
	Type int
	// Email 目标邮箱。
	Email string
	// ClientIP 请求方 IP，用于限流维度。
	ClientIP string
}

// EmailCaptchaSendResult 发送邮箱验证码结果。
type EmailCaptchaSendResult struct {
	// EmailID 验证码在存储中的索引 ID，需回传给校验接口。
	EmailID string
	// Cost 从进入用例到提交异步发送的耗时（与原日志字段一致）。
	Cost time.Duration
}
