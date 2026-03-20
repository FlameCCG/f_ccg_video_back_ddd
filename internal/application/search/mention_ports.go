package search

import (
	"context"
)

// MentionSearch @ 提及联想在 ES 侧的用户名/拼音匹配端口.
type MentionSearch interface {
	// Available 报告 ES 客户端是否就绪；false 时退化为本地字面匹配。
	Available() bool
	// MatchUserIDs 在给定候选用户 ID 集合内做用户名/拼音匹配，返回命中的用户 ID。
	MatchUserIDs(ctx context.Context, userIDs []uint, keyword string) ([]uint, error)
}

// MentionRelationQuery @ 提及候选集合查询端口（当前用户的关注列表 + 互关标记 + 粉丝数）.
type MentionRelationQuery interface {
	// FollowingWithFriendFlag 返回 userID 关注的用户，并标注是否互关。
	FollowingWithFriendFlag(ctx context.Context, userID uint) ([]MentionUser, error)
}
