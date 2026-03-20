package social

import (
	"context"
)

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
