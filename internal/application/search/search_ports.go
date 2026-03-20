package search

import (
	"context"
	"errors"
	"time"
)

// ErrUnexpectedMultiSearchSize 表示 multi-search 的响应条数与请求不匹配。
var ErrUnexpectedMultiSearchSize = errors.New("unexpected elasticsearch search response size")

// ErrESUnavailable 表示 Elasticsearch 客户端未初始化。
var ErrESUnavailable = errors.New("elasticsearch client not initialized")

// VideoHits 是一页视频检索命中及其总量.
type VideoHits struct {
	Total int64
	Items []SearchVideoHit
}

// UserHits 是一页用户检索命中及其总量.
type UserHits struct {
	Total int64
	Items []SearchUserHit
}

// CombinedHits 是一次 multi-search 同时取回的视频与用户命中.
type CombinedHits struct {
	Videos VideoHits
	Users  UserHits
}

// KeywordSearch 关键词检索端口。
//
// 索引名（search_videos / search_users）与查询 DSL 属于外部契约，由适配器保持逐字一致。
type KeywordSearch interface {
	// Available 报告 ES 客户端是否就绪；false 时应用层走 DB 回退。
	Available() bool
	// MultiSearch 一次请求同时检索视频与用户索引（综合搜索）。
	// 响应条数异常时返回 ErrUnexpectedMultiSearchSize。
	MultiSearch(ctx context.Context, opts SearchOptions, from int) (CombinedHits, error)
	// SearchVideos 仅检索视频索引（已发布过滤 + 排序 + 高亮）。
	SearchVideos(ctx context.Context, opts SearchOptions, from int) (VideoHits, error)
	// SearchUsers 仅检索用户索引（排序 + 高亮）。
	SearchUsers(ctx context.Context, opts SearchOptions, from int) (UserHits, error)
	// CountUsers 只取用户索引命中总量（size=0）。
	CountUsers(ctx context.Context, keyword string) (int64, error)
}

// VideoSearchMeta 是搜索结果回填用的库内视频元数据.
type VideoSearchMeta struct {
	ID             uint
	AuthorUsername string
	CreatedAt      time.Time
}

// SearchFallbackQuery ES 不可用时的搜索回退查询端口（读侧投影，直读 videos/users 表）.
type SearchFallbackQuery interface {
	// Available 报告数据库句柄是否就绪。
	Available() bool
	// SearchVideos 按标题/作者名模糊匹配已发布视频，返回一页命中与总量。
	SearchVideos(ctx context.Context, opts SearchOptions) ([]SearchVideoHit, int64, error)
	// SearchUsers 按用户名模糊匹配用户，返回一页命中与总量（含粉丝数聚合）。
	SearchUsers(ctx context.Context, opts SearchOptions) ([]SearchUserHit, int64, error)
	// CountUsers 统计用户名模糊命中总量。
	CountUsers(ctx context.Context, keyword string) (int64, error)
	// VideoMetaByIDs 批量读取视频作者名与创建时间，用于回填 ES 文档缺失字段。
	VideoMetaByIDs(ctx context.Context, ids []uint) ([]VideoSearchMeta, error)
}
