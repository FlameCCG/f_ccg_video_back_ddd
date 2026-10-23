package social

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/social"
)

// ---------------------------------------------------------------------------
// 读侧查询端口：实现落在本上下文的 infrastructure/persistence/social。
// 其中 users / user_confs / dynamics 的读取按 docs/architecture.md 第 4 节
// 「读侧投影允许直读其它上下文的表」执行；写侧一律经下方协作端口。
// ---------------------------------------------------------------------------

// 端口阅读：用户存在性、粉丝指标和列表投影由本地端口表达；隐私规则仍由 domain/social 决定。
// 保留的索引同步接口不表示关注写用例当前会同步调用它，实际用户文档更新还经过 River。

// RelationQuery 关注关系读侧端口。
type RelationQuery interface {
	// Exists 判定 userID 是否关注了 targetUserID。
	Exists(ctx context.Context, userID, targetUserID uint) bool
	// Relation 一次查询取回双向关注关系。
	Relation(ctx context.Context, userID, targetUserID uint) domain.Relation
	// Counts 统计关注数/粉丝数（withDynamic 为真时附带动态数）；查询失败返回零值。
	Counts(ctx context.Context, userID uint, withDynamic bool) domain.RelationCounts
	// FollowPrivacy 读取关注/粉丝列表公开性；缺省视为公开。
	FollowPrivacy(ctx context.Context, userID uint) (domain.FollowPrivacy, error)
	// ActionPrivacy 读取最近点赞/投币公开性；缺省视为公开。
	ActionPrivacy(ctx context.Context, userID uint) (domain.ActionPrivacy, error)
	// ListFocus 分页列出 userID 关注的用户。
	ListFocus(ctx context.Context, userID uint, page, pageSize int, keyword string) ([]SocialListUser, int64, error)
	// ListFans 分页列出关注了 userID 的用户。
	ListFans(ctx context.Context, userID uint, page, pageSize int, keyword string) ([]SocialListUser, int64, error)
}

// UserExistsQuery 目标用户存在性读侧端口（直读 users 表）。
type UserExistsQuery interface {
	// Exists 判定用户是否存在。
	Exists(ctx context.Context, userID uint) (bool, error)
}

// ---------------------------------------------------------------------------
// 跨上下文写侧协作端口：由 bootstrap 用 wire.Bind 绑定到对应上下文的实现。
// ---------------------------------------------------------------------------

// CreatorFansMetric 创作中心粉丝数指标端口（由 identity 上下文的创作中心服务实现）。
type CreatorFansMetric interface {
	// IncrFans 累加创作者当天的粉丝数指标（取关时 delta 为负）。
	IncrFans(ctx context.Context, creatorID uint, delta int64) error
}

// UserSearchSyncer 用户搜索文档同步端口（由 search 上下文实现）。
type UserSearchSyncer interface {
	// SyncUserDocuments 重建指定用户的搜索文档（粉丝数变化会影响排序权重）。
	SyncUserDocuments(ctx context.Context, userIDs ...uint) error
}
