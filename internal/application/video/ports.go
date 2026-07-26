package video

import (
	"context"
	"io"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
)

// ============================================================================
// 读侧查询端口（实现在 internal/infrastructure/persistence/video）
// ============================================================================

// VideoDetailQuery 视频详情读模型查询端口。
//
// 富嵌套读模型（作者/分区/标签/资源/分P）由 persistence 直接从 PO 组装，
// 不进入领域层（见 architecture.md 第 4 节）。
type VideoDetailQuery interface {
	// LoadPublished 读取已发布视频的详情 meta 与统计；不存在时返回 domain.ErrRecordNotFound。
	LoadPublished(ctx context.Context, videoID uint) (*DetailMeta, *domain.Stats, error)
	// LoadOwnerPreview 读取作者本人的非公开（私密/待审/定时）视频详情；
	// 不存在时返回 domain.ErrRecordNotFound。
	LoadOwnerPreview(ctx context.Context, videoID, viewerID uint) (*DetailMeta, *domain.Stats, error)
	// LoadAny 读取任意状态的视频详情（管理端使用）；不存在时返回 domain.ErrRecordNotFound。
	LoadAny(ctx context.Context, videoID uint) (*DetailMeta, *domain.Stats, error)
	// UserFlags 读取当前用户对视频的点赞/收藏/投币标记。
	UserFlags(ctx context.Context, videoID, userID uint) domain.UserFlags
}

// FeedQuery 首页/热门/排行/推荐的列表读模型查询端口。
type FeedQuery interface {
	// PreferredPartitionIDs 依据观看历史统计用户的偏好分区（按次数降序）。
	PreferredPartitionIDs(ctx context.Context, userID uint) []uint
	// FetchFeed 按分区过滤 + 历史去重拉取首页流；返回列表与总数。
	FetchFeed(ctx context.Context, partitionIDs, excludeVideoIDs []uint, page, pageSize int) ([]FeedItem, int64, error)
	// FetchByIDs 按 ID 列表读取列表项（仅已发布），返回 id → item 映射。
	FetchByIDs(ctx context.Context, videoIDs []uint) (map[uint]FeedItem, error)
	// FetchHotFromDB 用库内衰减热度表达式分页拉取综合热门。
	FetchHotFromDB(ctx context.Context, page, pageSize int) ([]FeedItem, int64, error)
	// CountPublished 统计已发布视频总数。
	CountPublished(ctx context.Context) (int64, error)
	// FetchRankFromDB 用库内综合分表达式分页拉取排行榜。
	FetchRankFromDB(ctx context.Context, offset, limit int) ([]FeedItem, error)
}

// RecommendQuery 相关推荐候选集查询端口。
type RecommendQuery interface {
	// BaseVideo 读取推荐基准视频的分区；视频不存在或未发布时返回 domain.ErrVideoNotPublished。
	BaseVideo(ctx context.Context, videoID uint) (partitionID uint, err error)
	// TagIDs 读取视频的标签 ID 列表。
	TagIDs(ctx context.Context, videoID uint) []uint
	// TagRelatedIDs 按共同标签数排序取候选视频 ID。
	TagRelatedIDs(ctx context.Context, videoID uint, tagIDs []uint, limit int) ([]uint, error)
	// PartitionRelatedIDs 按同分区热度取候选视频 ID。
	PartitionRelatedIDs(ctx context.Context, partitionID uint, limit int, exclude []uint) ([]uint, error)
	// HotRelatedIDs 按全局热度取候选视频 ID。
	HotRelatedIDs(ctx context.Context, limit int, exclude []uint) ([]uint, error)
	// FetchAnyStatusByIDs 按 ID 列表读取列表项（不过滤状态），返回 id → item 映射。
	FetchAnyStatusByIDs(ctx context.Context, videoIDs []uint) (map[uint]FeedItem, error)
}

// HistoryQuery 观看历史列表读模型查询端口。
type HistoryQuery interface {
	List(ctx context.Context, userID uint, page, pageSize int, keyword string) ([]HistoryItem, int64, error)
}

// FolderQuery 收藏夹读模型查询端口。
type FolderQuery interface {
	// ListWithCount 读取用户收藏夹列表（含视频数与「该视频是否已在此夹」）。
	ListWithCount(ctx context.Context, userID, videoID uint) ([]FolderWithCount, error)
	// ListVideos 读取收藏夹内的视频列表。
	ListVideos(ctx context.Context, folderID uint, page, pageSize, sort int) ([]FolderVideoItem, int64, error)
}

// DanmuQuery 弹幕读模型查询端口。
type DanmuQuery interface {
	// History 按播放时间区间读取历史弹幕。
	History(ctx context.Context, videoID, partID uint, start, end, limit int) ([]DanmuDTO, error)
	// RoomList 读取房间弹幕（分段模式按播放时间升序分页；历史模式按创建时间倒序取最近 N 条）。
	RoomList(ctx context.Context, params DanmuRoomQueryParams) ([]DanmuRoomItem, int64, error)
	// AdminList 管理端弹幕列表。
	AdminList(ctx context.Context, params AdminDanmuListParams) ([]AdminDanmuItem, int64, error)
	// CreatorList 创作中心弹幕列表（按作者用户名过滤）。
	CreatorList(ctx context.Context, authorUsername string, page, pageSize, sort int, keyword string) ([]CreatorDanmuItem, int64, error)
}

// AdminVideoQuery 管理端视频列表读模型查询端口。
type AdminVideoQuery interface {
	List(ctx context.Context, params AdminVideoListParams) ([]AdminVideoItem, int64, error)
}

// ReportQuery 管理端举报列表读模型查询端口。
type ReportQuery interface {
	AdminVideoReports(ctx context.Context, page, pageSize, status int) ([]AdminVideoReportItem, int64, error)
	AdminDanmuReports(ctx context.Context, page, pageSize, status int) ([]AdminDanmuReportItem, int64, error)
}

// UserVideoQuery 用户投稿列表读模型查询端口。
type UserVideoQuery interface {
	// ListByAuthor 按作者用户名分页读取投稿（isOwnerView 决定可见状态集合）。
	ListByAuthor(ctx context.Context, authorUsername string, isOwnerView bool, auditStatus, sort, page, pageSize int) ([]UserVideoRow, int64, error)
	// RecentActionVideos 读取用户最近点赞/投币的视频。
	RecentActionVideos(ctx context.Context, userID uint, action string, limit int) ([]UserVideoRow, error)
	// AggregateAuthorStats 汇总作者已发布视频的行内 views/likes（含视频 ID 列表以便叠加 Redis 权威计数）。
	AggregateAuthorStats(ctx context.Context, authorUsername string) ([]AuthorVideoStatRow, error)
	// ReportUserIDsByUsernames 按用户名批量解析用户 ID（读侧投影）。
	ReportUserIDsByUsernames(ctx context.Context, usernames []string) (map[string]uint, error)
	// UserProfile 读取用户 id/username/avatar（读侧投影，供通知渲染使用）。
	UserProfile(ctx context.Context, userID uint) (ActionUser, bool)
}

// ============================================================================
// 缓存端口（实现在 internal/infrastructure/cache/video）
// ============================================================================

// StatsCache 视频统计的 Redis 权威计数端口。
//
// Redis key 与 TTL、Lua 脚本语义、dirty 集合与 version key 均属外部契约，
// 由 infrastructure 适配器保持与原实现一致。
type StatsCache interface {
	// Available 报告 Redis 是否可用；不可用时应用层直接写 DB。
	Available() bool
	// Load 读取权威计数；未命中返回 (nil, false)。
	Load(ctx context.Context, videoID uint) (*domain.Stats, bool)
	// LoadBatch 批量读取权威计数（MGET），缺失项不出现在结果里。
	LoadBatch(ctx context.Context, videoIDs []uint) map[uint]*domain.Stats
	// Seed 仅在缺失时回填计数（SETNX + TTL）。
	Seed(ctx context.Context, videoID uint, stats *domain.Stats)
	// ApplyDelta 原子应用增量并标记 dirty + 递增 version；base 为冷缓存时的基线 payload。
	ApplyDelta(ctx context.Context, videoID uint, delta domain.StatsDelta, basePayload string) (*domain.Stats, error)
	// ApplyDeltaIfExists 仅在 key 已存在时应用增量；key 不存在返回 (nil, false, nil)。
	ApplyDeltaIfExists(ctx context.Context, videoID uint, delta domain.StatsDelta) (*domain.Stats, bool, error)
	// RawPayload 读取当前 stats key 的原始字符串；不存在返回 ("", false, nil)。
	RawPayload(ctx context.Context, videoID uint) (string, bool, error)
	// Clear 删除 stats / version / legacy synced key 并移出 dirty 集合。
	Clear(ctx context.Context, videoIDs ...uint)
	// IsTransientError 判断错误是否为 Redis 瞬时错误（可重试）。
	IsTransientError(err error) bool
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

// AuthorIDCache 视频作者 ID 的 Redis 缓存端口。
type AuthorIDCache interface {
	// Available 报告 Redis 是否可用。
	Available() bool
	// Load 读取作者 ID；未命中返回 (0, false)。
	Load(ctx context.Context, videoID uint) (uint, bool)
	// Store 写入作者 ID（长 TTL）。
	Store(ctx context.Context, videoID, authorID uint)
	// Invalidate 删除作者 ID 缓存。
	Invalidate(ctx context.Context, videoID uint)
}

// FolderListCache 收藏夹列表的 Redis 缓存端口。
type FolderListCache interface {
	Load(userID, videoID uint) ([]FolderWithCount, bool)
	Store(userID, videoID uint, items []FolderWithCount)
	Invalidate(userID uint)
}

// PartitionCache 可投稿分区列表的 Redis 缓存端口。
type PartitionCache interface {
	Load() ([]PartitionItem, bool)
	Store(items []PartitionItem)
	Invalidate()
}

// HotRankCache 视频热度 ZSet 端口。
type HotRankCache interface {
	// Available 报告 Redis 是否可用。
	Available() bool
	// IncrBy 累加热度分。
	IncrBy(ctx context.Context, videoID uint, score float64)
	// Card 读取 ZSet 基数。
	Card(ctx context.Context) (int64, error)
	// RevRange 倒序取排名区间的视频 ID。
	RevRange(ctx context.Context, start, stop int64) ([]uint, error)
	// Remove 从 ZSet 移除视频。
	Remove(ctx context.Context, videoIDs []uint)
}

// DanmuRecentCache 最近弹幕列表的 Redis 缓存端口。
type DanmuRecentCache interface {
	// Available 报告 Redis 是否可用。
	Available() bool
	// Push 追加一条弹幕并裁剪列表长度、续期 TTL。
	Push(ctx context.Context, dto *DanmuDTO)
	// Invalidate 删除某房间的最近弹幕缓存。
	Invalidate(ctx context.Context, videoID, partID uint)
}

// ScheduleStore 定时发布任务的 Redis ZSet 端口。
type ScheduleStore interface {
	// Available 报告 Redis 是否可用。
	Available() bool
	// Add 写入定时发布任务。
	Add(ctx context.Context, videoID uint, when time.Time) error
	// Remove 移除定时发布任务。
	Remove(ctx context.Context, videoID uint) error
	// Score 读取定时发布时间戳（Unix 秒）；无任务时返回 (0, false, nil)。
	Score(ctx context.Context, videoID uint) (float64, bool, error)
	// DueMembers 读取到期任务的原始 member 列表。
	DueMembers(ctx context.Context, nowUnix int64) ([]string, error)
	// RemoveMember 按原始 member 移除任务（用于清理非法 member）。
	RemoveMember(ctx context.Context, member string) error
}

// AuthorStatsCache 作者作品聚合统计的进程内短 TTL 缓存端口。
type AuthorStatsCache interface {
	Load(authorUsername string) (UserVideoStats, bool)
	Store(authorUsername string, stats UserVideoStats)
}

// UsernameCache 用户 ID → 用户名的进程内缓存端口。
type UsernameCache interface {
	Load(userID uint) (string, bool)
	Store(userID uint, username string)
	Delete(userID uint)
}

// ============================================================================
// 跨上下文端口（由 bootstrap 用 wire.Bind 绑定到对应上下文的实现）
// ============================================================================

// TranscodeDispatcher 转码任务投递端口。
//
// 绑定目标：media 上下文（原 service/mq_service）的转码任务生产者。
// 适配器负责把 TranscodeTask 映射为 pkg/mq.TranscodePayload 并 JSON 序列化后投递，
// 保证 Kafka topic 与消息结构与原实现逐字一致。
type TranscodeDispatcher interface {
	// PublishTranscodeTask 投递一条转码任务。
	PublishTranscodeTask(ctx context.Context, task TranscodeTask) error
	// MarkTranscodeQueued 把任务标记为「已入队」，供管理端进度查询展示。
	MarkTranscodeQueued(ctx context.Context, videoID, partID uint)
}

// TranscodeTask 转码任务载荷（字段与原 pkg/mq.TranscodePayload 一一对应）。
type TranscodeTask struct {
	ResolutionList []uint16
	StorageType    enum.VideoStorageType
	SourcePath     string
	OutputPath     string
	FileHash       string
	FileName       string
	VideoID        uint
	VideoPartID    uint
}

// TranscodeProgressReader 转码进度查询与订阅端口。
//
// 绑定目标：media 上下文（原 service/mq_service 的 MGetTranscodeProgressByVideoIDs /
// SubscribeTranscodeProgress）。
type TranscodeProgressReader interface {
	// MGetByVideoIDs 批量读取给定视频的进度快照。
	MGetByVideoIDs(ctx context.Context, videoIDs []uint) ([]TranscodeProgressItem, error)
	// Subscribe 订阅进度变更事件；返回的订阅需由调用方 Close。
	Subscribe(ctx context.Context) (TranscodeProgressSubscription, error)
	// IsTerminalStatus 判断进度状态是否为终态（succeeded / failed）。
	IsTerminalStatus(status string) bool
}

// TranscodeProgressSubscription 转码进度订阅。
type TranscodeProgressSubscription interface {
	// Events 返回进度事件通道；通道关闭表示订阅结束。
	Events() <-chan TranscodeProgressItem
	// Close 关闭订阅。
	Close() error
}

// TranscodeCanceller 转码取消通知端口。
//
// 绑定目标：media 上下文（原 service/transcode_cancel_service.NotifyVideoDeleted）。
type TranscodeCanceller interface {
	// NotifyVideoDeleted 广播视频已删除事件，取消在途转码任务。
	NotifyVideoDeleted(ctx context.Context, videoIDs []uint) error
}

// StorageDriver 对象存储驱动端口（原 service/storage_service.Driver 的视频侧子集）。
type StorageDriver interface {
	SaveChunk(ctx context.Context, fileHash, index string, size int64, reader io.Reader) error
	ListUploadedChunks(ctx context.Context, fileHash string) ([]string, error)
	GetCompletedFile(ctx context.Context, fileHash string) (string, bool, error)
	MergeChunks(ctx context.Context, fileHash, fileName string, totalChunks int) (string, error)
	SaveFile(ctx context.Context, relativePath string, size int64, reader io.Reader, contentType string) (string, error)
	DeleteStoredFile(ctx context.Context, storedPath string) error
	DeleteByPrefix(ctx context.Context, prefix string) (int64, error)
}

// StorageResolver 存储驱动解析与上传标识校验端口。
//
// 绑定目标：media 上下文（原 service/storage_service.ResolveDriver /
// NewLocalStorage / NewMinIOStorage / ValidateUploadID / ValidateChunkIndex）。
type StorageResolver interface {
	// Resolve 按站点配置解析当前生效的存储驱动。
	Resolve() (StorageDriver, error)
	// ResolveByType 按视频记录的存储类型解析驱动（硬删清理历史对象时使用）。
	ResolveByType(storageType enum.VideoStorageType) (StorageDriver, error)
	// ValidateUploadID 校验上传标识（fileHash）合法性。
	ValidateUploadID(id string) error
	// ValidateChunkIndex 校验分片索引合法性。
	ValidateChunkIndex(index string) error
}

// VideoSearcher 视频搜索端口。
//
// 绑定目标：search 上下文（原 service/search_service.SearchService）。
// 返回值使用 any，以保证响应 JSON 完全等于搜索上下文产出的结构。
type VideoSearcher interface {
	SearchVideos(ctx context.Context, opts VideoSearchOptions) (any, error)
	Suggest(ctx context.Context, prefix string, size int) (any, error)
	HotKeywords(ctx context.Context, page, pageSize int) (any, int64, error)
	RecordHotKeyword(ctx context.Context, keyword string)
}

// VideoSearchOptions 视频搜索参数（字段与原 search_service.SearchOptions 的视频侧子集一致）。
type VideoSearchOptions struct {
	Keyword    string
	Page       int
	PageSize   int
	VideoSort  int
	VideoOrder int
}

// VideoVectorIndexer 视频向量索引端口（最佳努力，失败仅记录日志）。
//
// 绑定目标：search 上下文（原 search_service.IndexVideoVectorBestEffort /
// DeleteVideoVectorBestEffort）。
type VideoVectorIndexer interface {
	IndexVideoVectorBestEffort(ctx context.Context, videoID uint)
	DeleteVideoVectorBestEffort(ctx context.Context, videoID uint)
}

// VectorIndexRebuilder 向量索引全量重建端口。
//
// 绑定目标：search 上下文（原 search_service.StartVideoVectorRebuild /
// VideoVectorRebuildStatus）。
// 已有任务在跑时约定返回 (false, nil)，由应用层转成
// ErrVectorRebuildInProgress 的错误文案，避免跨上下文共享哨兵错误。
type VectorIndexRebuilder interface {
	StartRebuild(opts VectorRebuildOptions) (started bool, err error)
	RebuildStatus() any
}

// VectorRebuildOptions 向量索引重建参数（与原 search_service.RebuildOptions 一致）。
type VectorRebuildOptions struct {
	// Recreate 为 true 时先删除索引再按当前配置重建。
	Recreate bool
	// Workers 并发向量化数；<=0 时由实现取默认值。
	Workers int
}

// ExpAwarder 经验变动端口。
//
// 绑定目标：identity 上下文的 *identity.ExpService。
type ExpAwarder interface {
	// WatchExp 每日观看视频加经验（每日一次幂等）。
	WatchExp(ctx context.Context, userID uint) error
	// LikeExp 点赞加经验（每日一次幂等）。
	LikeExp(ctx context.Context, userID uint) error
	// CoinExpForVideo 投币加经验（同一用户同一视频仅一次）。
	CoinExpForVideo(ctx context.Context, userID, videoID uint, coins int64) error
}

// CoinLedger 硬币账本端口。
//
// 绑定目标：identity 上下文的 *identity.CoinService。
// 在已有事务的 context 中调用 ApplyChange 时，identity 侧会复用同一事务
// （见 decisions.md D-010），因此三连/投币的跨上下文原子性与原实现一致。
type CoinLedger interface {
	// ApplyChange 增减用户硬币余额并写入流水。
	ApplyChange(ctx context.Context, userID uint, delta float64, reason string, dedupKey *string) error
	// IsInsufficient 判断错误是否为「硬币余额不足」。
	//
	// 显式暴露判定方法而非共享哨兵错误，避免 video 上下文 import identity 领域包。
	IsInsufficient(err error) bool
}

// CreatorAnalytics 创作中心指标累加端口。
//
// 绑定目标：identity 上下文的 *identity.CreatorAnalyticsService。
type CreatorAnalytics interface {
	IncrViews(ctx context.Context, creatorID uint, delta int64) error
	IncrComments(ctx context.Context, creatorID uint, delta int64) error
	IncrCoins(ctx context.Context, creatorID uint, delta int64) error
	IncrDanmu(ctx context.Context, creatorID uint, delta int64) error
	IncrFavorites(ctx context.Context, creatorID uint, delta int64) error
}

// VideoNotifier 通知端口。
//
// 绑定目标：notification 上下文的 *notification.Service（需补两个方法或加薄适配器）。
// 与原实现一致：写入通知行后失效对应接收者的角标缓存；失败仅记录日志。
type VideoNotifier interface {
	// NotifyVideoLike 视频被点赞通知。
	NotifyVideoLike(ctx context.Context, receiverID uint, actionUser ActionUser, videoID uint, videoTitle string)
	// NotifySystemBatch 批量系统通知（举报处理结果）。
	NotifySystemBatch(ctx context.Context, notices []SystemNotice)
}

// ActionUser 触发通知的操作用户信息。
type ActionUser struct {
	ID       uint
	Username string
	Avatar   string
}

// SystemNotice 一条系统通知。
type SystemNotice struct {
	ReceiverID       uint
	ActionUserID     uint
	ActionUserName   string
	ActionUserAvatar string
	Title            string
	Content          string
}

// CommentPurger 视频关联评论清理端口。
//
// 绑定目标：comment 上下文（原 service/video_service 直接删 comments / comment_likes）。
// 作者软删路径在事务内调用，comment 侧实现会复用 context 中的事务。
type CommentPurger interface {
	// PurgeByVideoIDs 删除给定视频的全部评论与评论点赞，返回删除计数。
	PurgeByVideoIDs(ctx context.Context, videoIDs []uint) (comments, commentLikes int64, err error)
}

// DynamicReadMarker 动态已读标记端口。
//
// 绑定目标：dynamic 上下文（原 dynamic_service.DynamicService.MaybeMarkReadByVideo）。
type DynamicReadMarker interface {
	MaybeMarkReadByVideo(ctx context.Context, userID, videoID uint) error
}
