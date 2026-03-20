package social

import (
	"context"

	socialentity "f_ccg_video_back_ddd/internal/domain/social/entity"
	socialvo "f_ccg_video_back_ddd/internal/domain/social/valueobject"
)

// RelationQuery 关注关系读侧端口。
type RelationQuery interface {
	// Exists 判定 userID 是否关注了 targetUserID。
	Exists(ctx context.Context, userID, targetUserID uint) bool
	// Relation 一次查询取回双向关注关系。
	Relation(ctx context.Context, userID, targetUserID uint) socialentity.Relation
	// Counts 统计关注数/粉丝数（withDynamic 为真时附带动态数）；查询失败返回零值。
	Counts(ctx context.Context, userID uint, withDynamic bool) socialentity.RelationCounts
	// FollowPrivacy 读取关注/粉丝列表公开性；缺省视为公开。
	FollowPrivacy(ctx context.Context, userID uint) (socialvo.FollowPrivacy, error)
	// ActionPrivacy 读取最近点赞/投币公开性；缺省视为公开。
	ActionPrivacy(ctx context.Context, userID uint) (socialvo.ActionPrivacy, error)
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
