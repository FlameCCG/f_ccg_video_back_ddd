package comment

import (
	"context"

	commentdomain "f_ccg_video_back_ddd/internal/domain/comment"
)

// Query 评论自身的读侧投影端口（实现在 infrastructure/persistence/comment）。
//
// 列表方法先 Count 再 Find，一次返回 total 与当页数据；管理端与创作中心在 total==0 时跳过 Find。
type Query interface {
	// ListRoots 一级评论分页列表（含置顶优先排序）。
	ListRoots(ctx context.Context, params RootListQuery) ([]CommentRow, int64, error)
	// ListReplies 某根评论下的回复分页列表（按创建时间升序）。
	ListReplies(ctx context.Context, params ReplyListQuery) ([]CommentRow, int64, error)
	// ListByAuthor 创作中心：本人视频下的评论分页列表。
	ListByAuthor(ctx context.Context, params CreatorListQuery) ([]CommentRow, int64, error)
	// AdminList 管理端评论分页列表。
	AdminList(ctx context.Context, params AdminListQuery) ([]CommentRow, int64, error)
	// CountByVideo 统计某视频的评论总数（含回复）。
	CountByVideo(ctx context.Context, videoID uint) (int64, error)
	// ParentBrief 读取同一对象下的父评论投影（user_id / root_id / 用户名）；
	// 不存在时返回 (nil, nil)。
	ParentBrief(ctx context.Context, parentID, videoID, dynamicID uint) (*commentdomain.ParentBrief, error)
}
