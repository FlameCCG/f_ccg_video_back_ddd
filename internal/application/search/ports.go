package search

import (
	"context"
	"errors"
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// 端口阅读：关键词索引、SQL 回退、联想缓存、热词、向量源数据与嵌入能力使用不同端口。
// Available 通常检查是否已配置句柄；实际网络调用仍可能失败，应结合用例的回退分支理解。

// ErrUnexpectedMultiSearchSize 表示 multi-search 返回的响应条数与请求不匹配。
// 与原 service/search_service.Search 的 "unexpected elasticsearch search response size" 等价。
var ErrUnexpectedMultiSearchSize = errors.New("unexpected elasticsearch search response size")

// ErrVectorSourceNotFound 表示向量索引源视频在库中不存在（等价原 gorm.ErrRecordNotFound 分支）。
var ErrVectorSourceNotFound = errors.New("video vector source not found")

// ErrESUnavailable 表示 Elasticsearch 客户端未初始化。
// 与原 service/search_service.fetchFieldSuggestions 的 "elasticsearch client not initialized" 等价。
var ErrESUnavailable = errors.New("elasticsearch client not initialized")

// ---------------- Elasticsearch 检索端口（实现：infrastructure/search） ----------------

// VideoHits 是一页视频检索命中及其总量.
type VideoHits struct {
	Total int64
	Items []VideoHit
}

// UserHits 是一页用户检索命中及其总量.
type UserHits struct {
	Total int64
	Items []UserHit
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
	MultiSearch(ctx context.Context, opts Options, from int) (CombinedHits, error)
	// SearchVideos 仅检索视频索引（已发布过滤 + 排序 + 高亮）。
	SearchVideos(ctx context.Context, opts Options, from int) (VideoHits, error)
	// SearchUsers 仅检索用户索引（排序 + 高亮）。
	SearchUsers(ctx context.Context, opts Options, from int) (UserHits, error)
	// CountUsers 只取用户索引命中总量（size=0）。
	CountUsers(ctx context.Context, keyword string) (int64, error)
}

// SuggestSearch 搜索联想端口。
//
// docType 取 DocTypeVideo / DocTypeUser / DocTypeTag，适配器据此选择索引、
// 字段集合与「仅已发布」过滤条件。
type SuggestSearch interface {
	// Available 报告 ES 客户端是否就绪。
	Available() bool
	// FieldSuggest 基于 text/keyword/pinyin 倒排索引取联想候选，不依赖 completion FST。
	FieldSuggest(ctx context.Context, docType, prefix string, size int) ([]SuggestionItem, error)
}

// MentionSearch @ 提及联想在 ES 侧的用户名/拼音匹配端口.
type MentionSearch interface {
	// Available 报告 ES 客户端是否就绪；false 时退化为本地字面匹配。
	Available() bool
	// MatchUserIDs 在给定候选用户 ID 集合内做用户名/拼音匹配，返回命中的用户 ID。
	MatchUserIDs(ctx context.Context, userIDs []uint, keyword string) ([]uint, error)
}

// UserSearchDocument 是写入 search_users 索引的用户文档（字段名由适配器映射为索引字段）.
type UserSearchDocument struct {
	ID              uint
	Username        string
	Description     string
	Avatar          string
	Level           int
	CreatedAt       time.Time
	UpdatedAt       time.Time
	FollowerCount   int64
	FollowUsernames []string
	FriendUsernames []string
}

// UserDocumentIndexer 用户搜索文档写入端口.
type UserDocumentIndexer interface {
	// Available 报告 ES 客户端是否就绪；false 时同步直接跳过（与原实现一致）。
	Available() bool
	// BulkIndexUsers 批量覆盖写入用户文档。
	BulkIndexUsers(ctx context.Context, docs []UserSearchDocument) error
}

// ---------------- 读侧 SQL 投影端口（实现：infrastructure/persistence/search） ----------------

// VideoSearchMeta 是搜索结果回填用的库内视频元数据.
type VideoSearchMeta struct {
	ID             uint
	AuthorUsername string
	CreatedAt      time.Time
}

// FallbackQuery ES 不可用时的搜索回退查询端口（读侧投影，直读 videos/users 表）.
type FallbackQuery interface {
	// Available 报告数据库句柄是否就绪。
	Available() bool
	// SearchVideos 按标题/作者名模糊匹配已发布视频，返回一页命中与总量。
	SearchVideos(ctx context.Context, opts Options) ([]VideoHit, int64, error)
	// SearchUsers 按用户名模糊匹配用户，返回一页命中与总量（含粉丝数聚合）。
	SearchUsers(ctx context.Context, opts Options) ([]UserHit, int64, error)
	// CountUsers 统计用户名模糊命中总量。
	CountUsers(ctx context.Context, keyword string) (int64, error)
	// VideoMetaByIDs 批量读取视频作者名与创建时间，用于回填 ES 文档缺失字段。
	VideoMetaByIDs(ctx context.Context, ids []uint) ([]VideoSearchMeta, error)
}

// SuggestFallbackQuery ES 不可用时的联想回退查询端口.
type SuggestFallbackQuery interface {
	// Available 报告数据库句柄是否就绪。
	Available() bool
	// VideoTitles 取已发布视频标题（前缀命中优先，按最新去重标题排序）。
	VideoTitles(ctx context.Context, prefix string, size int) ([]string, error)
	// Usernames 取用户名（前缀命中优先）。
	Usernames(ctx context.Context, prefix string, size int) ([]string, error)
	// TagNames 取挂在已发布视频上的标签名（前缀命中优先）。
	TagNames(ctx context.Context, prefix string, size int) ([]string, error)
}

// HotVideoTitle 是热词回退采样得到的视频标题与播放量.
type HotVideoTitle struct {
	Title string
	Views int64
}

// HotKeywordFallbackQuery Redis 热搜词榜为空时的回退查询端口.
type HotKeywordFallbackQuery interface {
	// Available 报告数据库句柄是否就绪。
	Available() bool
	// HotVideoTitles 按播放量取热门已发布视频标题样本。
	HotVideoTitles(ctx context.Context, sampleSize int) ([]HotVideoTitle, error)
}

// MentionRelationQuery @ 提及候选集合查询端口（当前用户的关注列表 + 互关标记 + 粉丝数）.
type MentionRelationQuery interface {
	// FollowingWithFriendFlag 返回 userID 关注的用户，并标注是否互关。
	FollowingWithFriendFlag(ctx context.Context, userID uint) ([]MentionUser, error)
}

// UserSearchDocumentQuery 用户搜索文档的数据来源端口（用户行 + 关系统计）.
type UserSearchDocumentQuery interface {
	// LoadDocuments 按给定顺序装载用户文档；库中不存在的 ID 直接跳过。
	LoadDocuments(ctx context.Context, userIDs []uint) ([]UserSearchDocument, error)
}

// VideoVectorSource 是构建视频向量文档所需的库内数据.
type VideoVectorSource struct {
	ID          uint
	Title       string
	Description string
	Cover       string
	Partition   string
	Status      enum.VideoStatus
	CreatedAt   time.Time
	Tags        []string
	PartTitles  []string
}

// VideoVectorSourceQuery 向量索引源数据查询端口.
type VideoVectorSourceQuery interface {
	// Available 报告数据库句柄是否就绪。
	Available() bool
	// Load 读取视频及其标签/分区/分P；不存在时返回 ErrVectorSourceNotFound。
	Load(ctx context.Context, videoID uint) (VideoVectorSource, error)
}

// VideoVectorRebuildSource 是向量索引全量回填的主键游标读端口。
type VideoVectorRebuildSource interface {
	// Available 报告数据库句柄是否就绪。
	Available() bool
	// CountPublished 统计当前已发布视频总数。
	CountPublished(ctx context.Context) (int64, error)
	// PublishedAfter 按主键游标读取一页已发布视频及向量化所需的全部关联数据。
	PublishedAfter(ctx context.Context, cursor uint, limit int) ([]VideoVectorSource, error)
}

// ---------------- 缓存端口（实现：infrastructure/cache/search） ----------------

// HotKeywordEntry 是 Redis 热搜词榜的一个成员及其分数.
type HotKeywordEntry struct {
	Member string
	Score  float64
}

// HotKeywordStore 热搜词榜存储端口。
//
// Redis key（search:hot:keywords:zset）与「只增不过期」的语义属于外部契约。
type HotKeywordStore interface {
	// Available 报告 Redis 客户端是否就绪。
	Available() bool
	// Card 返回榜单成员总数。
	Card(ctx context.Context) (int64, error)
	// RevRangeWithScores 按分数倒序取 [start,stop] 闭区间成员。
	RevRangeWithScores(ctx context.Context, start, stop int64) ([]HotKeywordEntry, error)
	// Incr 给关键词热度加一；失败静默忽略（best-effort）。
	Incr(ctx context.Context, keyword string)
}

// SuggestionCache 联想结果的进程内短 TTL 缓存端口.
type SuggestionCache interface {
	// Load 读取缓存；未命中或已过期返回 false。
	Load(key string) ([]SuggestionItem, bool)
	// Store 写入缓存并刷新过期时间。
	Store(key string, items []SuggestionItem)
}

// ---------------- 向量索引端口（实现：infrastructure/search） ----------------

// VectorConfig 是一次向量检索/索引所使用的运行期配置快照.
type VectorConfig struct {
	// Index 向量索引名。
	Index string
	// TopK 向量召回上限。
	TopK int
	// EmbeddingModel 向量模型名。
	EmbeddingModel string
	// EmbeddingAPIKey 方舟 API Key。
	EmbeddingAPIKey string
	// EmbeddingDimensions 向量维度。
	EmbeddingDimensions int
	// TimeoutSec 向量模型调用超时（秒）；<=0 时适配器取默认值。
	TimeoutSec int
	// ImageMode 控制封面图参与索引向量化：auto / on / off。
	ImageMode string
	// AssetBaseURL 用于把相对封面路径补成方舟可访问的绝对地址。
	AssetBaseURL string
	// LocalStorageRoot 是启用本地存储时的根目录；为空表示不可从磁盘读取封面。
	LocalStorageRoot string
	// RRFRankConstant 是向量与关键词召回做 RRF 融合时的排名常数。
	RRFRankConstant int
	// MinSimilarity 是向量召回最低余弦相似度；<=0 表示不设置阈值。
	MinSimilarity float64
}

// EmbedInput 描述一次多模态向量检索输入。
// 文本与图片可同时提供，基础设施适配器会把二者融合进同一向量空间。
type EmbedInput struct {
	Text     string
	ImageURL string
}

// IsZero 判断多模态输入是否同时缺少文本和图片。
func (in EmbedInput) IsZero() bool {
	return strings.TrimSpace(in.Text) == "" && strings.TrimSpace(in.ImageURL) == ""
}

// VideoVectorIndex 视频向量索引端口（ES8 + eino-ext）.
type VideoVectorIndex interface {
	// Available 报告 ES8 客户端是否就绪。
	Available() bool
	// SearchIDs 以文本、图片或二者组合做混合召回，返回去重后的视频 ID（最多 limit 个）。
	SearchIDs(ctx context.Context, cfg VectorConfig, input EmbedInput, limit int) ([]uint, error)
	// Store 覆盖写入一个视频的向量文档。
	Store(ctx context.Context, cfg VectorConfig, source VideoVectorSource) error
	// Delete 删除一个视频的向量文档；文档不存在视为成功。
	Delete(ctx context.Context, cfg VectorConfig, videoID uint) error
}

// VideoVectorIndexLifecycle 是全量回填所需的索引运维端口。
type VideoVectorIndexLifecycle interface {
	// Available 报告 ES8 客户端是否就绪。
	Available() bool
	// Prepare 确保索引存在；recreate 为 true 时先删除再按当前维度创建。
	Prepare(ctx context.Context, cfg VectorConfig, recreate bool) error
	// Refresh 让本次写入立即可检索。
	Refresh(ctx context.Context, cfg VectorConfig) error
}
