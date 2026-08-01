// Package identity 是身份上下文的应用层：用例服务、DTO（含读模型）与查询端口。
//
// 本包不依赖 gin / gorm / redis：数据访问一律经端口，实现在 infrastructure。
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

// AdminUserListItem 管理端用户列表项（读模型，json tag 与原实现一致）.
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

// CoinRecordItem 硬币流水列表项（读模型）.
type CoinRecordItem struct {
	ID        uint      `json:"id"`
	Delta     float64   `json:"delta"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"createdAt"`
}

// ExpRecordItem 经验流水列表项（读模型）.
type ExpRecordItem struct {
	ID        uint      `json:"id"`
	Delta     int64     `json:"delta"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"createdAt"`
}

// LoginRecordItem 登录记录列表项（读模型）.
type LoginRecordItem struct {
	ID        uint      `json:"id"`
	IP        string    `json:"ip"`
	Addr      string    `json:"addr"`
	UserAgent string    `json:"userAgent"`
	LoginType string    `json:"loginType"`
	CreatedAt time.Time `json:"createdAt"`
}

// CreatorOverview 创作中心六项总览（读模型）.
type CreatorOverview struct {
	Fans      int64 `json:"fans"`
	Views     int64 `json:"views"`
	Comments  int64 `json:"comments"`
	Coins     int64 `json:"coins"`
	Danmu     int64 `json:"danmu"`
	Favorites int64 `json:"favorites"`
}

// CreatorAnalyticsTrendResult 创作中心趋势结果（读模型）.
type CreatorAnalyticsTrendResult struct {
	Range  string   `json:"range"`
	Type   string   `json:"type"`
	X      []string `json:"x"`
	Values []int64  `json:"values"`
	Total  int64    `json:"total"`
}

// CreatorVideoAggregate 作者已发布视频的统计汇总（读模型，含 Redis 统计缓存优先）.
type CreatorVideoAggregate struct {
	Views     int64
	Comments  int64
	Coins     int64
	Danmu     int64
	Favorites int64
}

// 创作中心指标类型（与原 service/user_service/creator_analytics_service.go 一致）。
const (
	// CreatorAnalyticsTypeFans 粉丝数指标.
	CreatorAnalyticsTypeFans = "fans"
	// CreatorAnalyticsTypeViews 播放量指标.
	CreatorAnalyticsTypeViews = "views"
	// CreatorAnalyticsTypeComments 评论量指标.
	CreatorAnalyticsTypeComments = "comments"
	// CreatorAnalyticsTypeCoins 投币量指标.
	CreatorAnalyticsTypeCoins = "coins"
	// CreatorAnalyticsTypeDanmu 弹幕量指标.
	CreatorAnalyticsTypeDanmu = "danmu"
	// CreatorAnalyticsTypeFavorites 收藏量指标.
	CreatorAnalyticsTypeFavorites = "favorites"
)
