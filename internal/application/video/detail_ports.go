package video

import (
	"context"

	videodomain "f_ccg_video_back_ddd/internal/domain/video"
)

// VideoDetailQuery 视频详情读模型查询端口。
//
// 富嵌套读模型（作者/分区/标签/资源/分P）由 persistence 直接从 PO 组装，
// 不进入领域层（见 architecture.md 第 4 节）。
type VideoDetailQuery interface {
	// LoadPublished 读取已发布视频的详情 meta 与统计；不存在时返回 domain.ErrRecordNotFound。
	LoadPublished(ctx context.Context, videoID uint) (*DetailMeta, *videodomain.Stats, error)
	// LoadOwnerPreview 读取作者本人的非公开（私密/待审/定时）视频详情；
	// 不存在时返回 domain.ErrRecordNotFound。
	LoadOwnerPreview(ctx context.Context, videoID, viewerID uint) (*DetailMeta, *videodomain.Stats, error)
	// LoadAny 读取任意状态的视频详情（管理端使用）；不存在时返回 domain.ErrRecordNotFound。
	LoadAny(ctx context.Context, videoID uint) (*DetailMeta, *videodomain.Stats, error)
	// UserFlags 读取当前用户对视频的点赞/收藏/投币标记。
	UserFlags(ctx context.Context, videoID, userID uint) videodomain.UserFlags
}

// DetailMetaCache 视频详情 meta 的 Redis 缓存端口（generation CAS 防旧快照回填）。
type DetailMetaCache interface {
	// Available 报告 Redis 是否可用。
	Available() bool
	// Load 读取详情 meta；未命中返回 (nil, false)。
	Load(ctx context.Context, videoID uint) (*DetailMeta, bool)
	// Generation 读取 generation 快照（必须在读库之前捕获）。
	Generation(ctx context.Context, videoID uint) int64
	// StoreIfGenerationMatch 仅在 generation 未变化时写入 meta。
	StoreIfGenerationMatch(ctx context.Context, videoID uint, meta *DetailMeta, expectedGen int64)
	// Invalidate 删除 meta 并 bump generation。
	Invalidate(ctx context.Context, videoID uint)
	// Delete 仅删除 meta key（不 bump generation），用于审核流程的历史行为兼容。
	Delete(ctx context.Context, videoID uint)
}
