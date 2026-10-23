package comment

import (
	"context"

	domain "f_ccg_video_back_ddd/internal/domain/comment"
)

// 端口阅读：评论列表和评论对象/用户事实是读侧能力，通知、视频计数和热度是跨上下文协作能力。
// 方法的输入输出采用本上下文类型，提供方差异由 bootstrap/adapters 翻译。

// Query 评论自身的读侧投影端口（实现在 infrastructure/persistence/comment）。
//
// 列表方法一次性返回 total 与当页数据，SQL 执行顺序与原实现一致
// （先 Count 再 Find；管理端与创作中心在 total==0 时不再执行 Find）。
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
	ParentBrief(ctx context.Context, parentID, videoID, dynamicID uint) (*domain.ParentBrief, error)
}

// RelatedQuery 关联对象的读侧投影端口。
//
// 这些投影直读 users / videos / dynamics / user_focus 表：按 docs/architecture.md 第 4 节，
// 读侧投影允许直读其它上下文的表，实现放在消费方（本上下文）的 infrastructure；
// 写侧（视频计数、通知落库）一律经下方的跨上下文端口。
type RelatedQuery interface {
	// UserProfile 读取 id/username/avatar；不存在时返回 (nil, nil)。
	UserProfile(ctx context.Context, userID uint) (*UserProfile, error)
	// UserProfileByUsername 按用户名读取 id/username/avatar；不存在时返回 (nil, nil)。
	UserProfileByUsername(ctx context.Context, username string) (*UserProfile, error)
	// UsernameByID 读取用户名；不存在或查询失败时返回空串（与原实现忽略错误一致）。
	UsernameByID(ctx context.Context, userID uint) string
	// PublishedVideoBrief 读取已发布视频的 id/title/author_username；不存在时返回 (nil, nil)。
	PublishedVideoBrief(ctx context.Context, videoID uint) (*VideoBrief, error)
	// VideoBrief 读取视频 id/title（不限状态）；不存在时返回 (nil, nil)。
	VideoBrief(ctx context.Context, videoID uint) (*VideoBrief, error)
	// VideoAuthorRef 读取视频及其作者用户 ID；视频不存在时返回 (nil, nil)。
	VideoAuthorRef(ctx context.Context, videoID uint) (*VideoAuthorRef, error)
	// DynamicBrief 读取动态 id/user_id；不存在时返回 (nil, nil)。
	DynamicBrief(ctx context.Context, dynamicID uint) (*DynamicBrief, error)
	// AllowedAtUserIDs 过滤出与操作者互相关注（任一方向）的候选 @ 用户；
	// 查询失败时返回 nil（与原实现忽略错误一致）。
	AllowedAtUserIDs(ctx context.Context, actorID uint, candidateIDs []uint) []uint
}

// LikeCache 评论点赞缓存端口（Redis 实现在 infrastructure/cache/comment）。
//
// Redis key 前缀、TTL 与 Lua 原子切换脚本属于外部契约，由适配器保持与原实现一致。
type LikeCache interface {
	// Available 报告底层 Redis 是否可用。
	Available() bool
	// Complete 判断某评论的 count 与 users set 是否成套（半残视为不完整）。
	Complete(ctx context.Context, commentID uint) (bool, error)
	// Write 以给定点赞用户全量重写 set + count（同一 TTL）。
	Write(ctx context.Context, commentID uint, userIDs []uint) error
	// Clear 删除 count/set 两个 key（不写入 count=0）。
	Clear(ctx context.Context, commentID uint) error
	// Drop 尽力删除 count/set 两个 key，失败仅记录日志（用于写后失效）。
	Drop(ctx context.Context, commentID uint)
	// Toggle 用 Lua 原子切换点赞状态，返回最新点赞数与是否已点赞，并写入回写队列。
	Toggle(ctx context.Context, commentID, userID uint) (int64, bool, error)
	// Counts 批量读取点赞数；整体失败返回 (nil, 全部 ID)，部分缺失返回命中项与缺失 ID。
	Counts(ctx context.Context, commentIDs []uint) (map[uint]int64, []uint)
	// LikedFlags 批量读取点赞状态；整体失败返回 (nil, 全部 ID)，缓存不完整项记为缺失。
	LikedFlags(ctx context.Context, commentIDs []uint, userID uint) (map[uint]bool, []uint)
	// ReplyCounts 批量读取回复数缓存；缺失项由调用方用 DB 字段兜底。
	ReplyCounts(ctx context.Context, commentIDs []uint) map[uint]int64
}

// Notifier 站内通知写入端口（跨上下文：notification 写侧）。
//
// 由 notification 上下文的应用服务在 bootstrap 中绑定实现。
// 拆成「批量写入 + 失效计数」两步，是为了与原实现一致：
// 一次批量 INSERT，随后按去重后的接收者逐个失效角标缓存。
type Notifier interface {
	// CreateNotices 批量写入通知（一次插入，失败则整批不生效）。
	CreateNotices(ctx context.Context, notices []Notice) error
	// InvalidateCount 失效指定接收者的角标计数缓存。
	InvalidateCount(receiverID int)
}

// VideoCommentCounter 视频评论数增减端口（跨上下文：video 写侧）。
//
// 对应原 video_service.UpdateCommentCount：更新视频统计缓存并累加创作中心评论指标，
// 失败只记录日志、不返回错误。
type VideoCommentCounter interface {
	UpdateCommentCount(ctx context.Context, videoID uint, delta int64)
}

// VideoHotScoreRecorder 视频热度打点端口（跨上下文：video 写侧）。
//
// 对应原 video_service.RecordHotScoreForComment：按评论权重更新热度榜 ZSet。
type VideoHotScoreRecorder interface {
	RecordHotScoreForComment(ctx context.Context, videoID uint, delta int64)
}
