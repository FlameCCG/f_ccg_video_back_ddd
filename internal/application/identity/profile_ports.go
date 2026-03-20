package identity

import (
	"context"
)

// UserRelationCounts 关注/粉丝/动态计数。
type UserRelationCounts struct {
	FocusCount   int64
	FansCount    int64
	DynamicCount int64
}

// UserActionPrivacy 最近点赞/投币公开开关。
type UserActionPrivacy struct {
	Exists        bool
	OpenLikeVideo bool
	OpenCoinVideo bool
}

// UserRelationQuery 关注关系与隐私开关查询端口。
type UserRelationQuery interface {
	// RelationCounts 一次 SQL 取关注数/粉丝数（withDynamic 为 true 时附带动态数）。
	RelationCounts(ctx context.Context, userID uint, withDynamic bool) UserRelationCounts
	// DynamicCount 单独统计用户动态数。
	DynamicCount(ctx context.Context, userID uint) int64
	// ActionPrivacy 读取最近点赞/投币是否公开。
	ActionPrivacy(ctx context.Context, userID uint) (UserActionPrivacy, error)
}

// UserRoleQuery 用户角色查询端口。
type UserRoleQuery interface {
	// RoleIDByName 按角色名读取角色 ID；不存在时返回错误。
	RoleIDByName(ctx context.Context, name string) (uint, error)
	// HasRole 判断用户是否拥有指定角色。
	HasRole(ctx context.Context, userID, roleID uint) (bool, error)
	// LoadRoles 加载用户角色列表（OAuth 登录链路的可用性校验）。
	LoadRoles(ctx context.Context, userID uint) ([]UserRoleItem, error)
	// RoleMap 返回用户「角色 ID → 角色名」映射；无角色时返回 nil。
	RoleMap(ctx context.Context, userID uint) (map[uint]string, error)
}

// UserRoleItem 用户角色读模型。
type UserRoleItem struct {
	ID   uint
	Name string
}

// UserConfView 用户配置读模型。
//
// LikeTags 保持数据库中的原始 JSON 字节，作为 GET /user/info/conf 的响应内容。
type UserConfView struct {
	LikeTags      []byte `json:"likeTags"`
	BannerID      uint   `json:"bannerId"`
	BannerURL     string `json:"bannerUrl"`
	OpenCollect   bool   `json:"openCollect"`
	OpenFans      bool   `json:"openFans"`
	OpenFollow    bool   `json:"openFollow"`
	OpenLikeVideo bool   `json:"openLikeVideo"`
	OpenCoinVideo bool   `json:"openCoinVideo"`
}

// UserConfQuery 用户配置读侧端口。
type UserConfQuery interface {
	// View 读取用户配置读模型；不存在时返回 (nil, nil)。
	View(ctx context.Context, userID uint) (*UserConfView, error)
}
