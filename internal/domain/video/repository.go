package video

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// VideoRepository 视频聚合写侧仓储（接口所有权归领域层）。
type VideoRepository interface {
	// FindByID 按主键读取视频；不存在时返回 (nil, nil)。
	FindByID(ctx context.Context, videoID uint) (*Video, error)
	// FindByIDAndAuthorForUpdate 在调用方事务内锁定视频，直到本次改稿提交或回滚。
	// 不存在时返回 ErrRecordNotFound；调用方必须传入 tx.Manager 的事务 context。
	FindByIDAndAuthorForUpdate(ctx context.Context, videoID uint, authorUsername string) (*Video, error)
	// EnsurePublished 校验视频存在且处于已发布状态；否则返回 ErrRecordNotFound。
	EnsurePublished(ctx context.Context, videoID uint) error
	// Exists 判断视频行是否存在（不限状态）。
	Exists(ctx context.Context, videoID uint) (bool, error)
	// PublishedStats 读取已发布视频的统计行；不存在时返回 ErrRecordNotFound。
	PublishedStats(ctx context.Context, videoID uint) (*Stats, error)
	// ApplyStatsDelta 在 MySQL 上以「不小于 0」表达式应用统计增量（仅已发布视频）。
	ApplyStatsDelta(ctx context.Context, videoID uint, delta StatsDelta) error
	// AuthorIDByVideoID 读取视频作者的用户 ID；作者缺失时返回 ErrVideoAuthorNotFound。
	AuthorIDByVideoID(ctx context.Context, videoID uint) (uint, error)
	// PublishedAuthorIDByVideoID 读取已发布视频作者的用户 ID（供事务内互动流程使用）。
	PublishedAuthorIDByVideoID(ctx context.Context, videoID uint) (uint, error)
	// AuthorUsernameByVideoID 读取视频的作者用户名；视频不存在时返回 ErrRecordNotFound。
	AuthorUsernameByVideoID(ctx context.Context, videoID uint) (string, error)
	// UsernameByUserID 读取用户名（读侧投影，允许直读 users 表，见 decisions.md D-011）。
	UsernameByUserID(ctx context.Context, userID uint) (string, error)
	// UserIDByUsername 读取用户 ID 与规范化后的用户名（读侧投影）。
	UserIDByUsername(ctx context.Context, username string) (uint, string, error)
	// UpdateStatus 无条件更新视频状态。
	UpdateStatus(ctx context.Context, videoID uint, status enum.VideoStatus) error
	// UpdateStatusFrom 仅当当前状态等于 from 时更新为 to，返回影响行数。
	UpdateStatusFrom(ctx context.Context, videoID uint, from, to enum.VideoStatus) (int64, error)
	// UpdateStatusBatch 批量更新视频状态，返回影响行数。
	UpdateStatusBatch(ctx context.Context, videoIDs []uint, status enum.VideoStatus) (int64, error)
	// UpdateStatusBatchFrom 批量条件更新视频状态，返回影响行数。
	UpdateStatusBatchFrom(ctx context.Context, videoIDs []uint, from, to enum.VideoStatus) (int64, error)
	// IDsByAuthor 读取该作者名下的全部视频 ID（用于批量失效详情缓存）。
	IDsByAuthor(ctx context.Context, authorUsername string) ([]uint, error)
	// IDsByStatus 过滤出指定状态的视频 ID。
	IDsByStatus(ctx context.Context, videoIDs []uint, status enum.VideoStatus) ([]uint, error)
	// IDsByPartition 列出某分区下的全部视频 ID。
	IDsByPartition(ctx context.Context, partitionID uint) ([]uint, error)
	// CountByPartition 统计某分区下的视频数量。
	CountByPartition(ctx context.Context, partitionID uint) (int64, error)
	// SaveRevision 保存已锁定、经 Video.Revise 校验的改稿快照；不覆盖计数与媒体资源。
	SaveRevision(ctx context.Context, video *Video) error
	// ReplaceTags 覆盖视频的标签关联（tags 为空表示清空关联）。
	ReplaceTags(ctx context.Context, videoID uint, tags []Tag) error
	// TagsByVideoID 读取视频当前关联的标签。
	TagsByVideoID(ctx context.Context, videoID uint) ([]Tag, error)
	// FindTagsByIDs 按 ID 批量读取标签（无匹配时返回非 nil 空切片，与 GORM Find 语义一致）。
	FindTagsByIDs(ctx context.Context, tagIDs []uint) ([]Tag, error)
	// CreateOrGetTag 创建标签（若已存在同名标签则返回现有）。
	CreateOrGetTag(ctx context.Context, name string) (*Tag, error)
	// Persist 在单事务内落库一次投稿（视频 + 标签 + 分P + 初始资源）。
	Persist(ctx context.Context, draft *PublishDraft) error
	// HardDelete 物理删除视频及其全部关联数据，返回各表删除计数与待清理的存储目标。
	HardDelete(ctx context.Context, videoIDs []uint) (*HardDeleteResult, []StorageDeleteTarget, error)
	// StoragePathStillReferenced 判断对象路径是否仍被其它视频/资源引用，兼容历史完整 URL/bucket 前缀。
	StoragePathStillReferenced(ctx context.Context, storedPath string) (bool, error)
}

// HardDeleteResult 物理删除的各表影响行数（json tag 与原 AdminVideoDeleteResult 一致）。
type HardDeleteResult struct {
	Videos              int64 `json:"videos"`
	VideoParts          int64 `json:"videoParts"`
	PartResources       int64 `json:"partResources"`
	Resources           int64 `json:"resources"`
	Comments            int64 `json:"comments"`
	CommentLikes        int64 `json:"commentLikes"`
	Danmus              int64 `json:"danmus"`
	DanmuLikes          int64 `json:"danmuLikes"`
	Favorites           int64 `json:"favorites"`
	Likes               int64 `json:"likes"`
	Coins               int64 `json:"coins"`
	Histories           int64 `json:"histories"`
	Reports             int64 `json:"reports"`
	DanmuReports        int64 `json:"danmuReports"`
	Tags                int64 `json:"tags"`
	StorageFiles        int64 `json:"storageFiles"`
	StorageFilesSkipped int64 `json:"storageFilesSkipped"`
	StorageDeleteFailed int64 `json:"storageDeleteFailed"`
}

// StorageDeleteTarget 待清理的对象存储目标。
type StorageDeleteTarget struct {
	StorageType enum.VideoStorageType
	Path        string
	// DashDirPrefix 非空时先按前缀清理整个 DASH 目录，再删 Path 指向的单个文件。
	DashDirPrefix string
}

// LikeRepository 点赞关系仓储。
type LikeRepository interface {
	// Flip 事务内翻转点赞关系；返回 likes 增量（+1 点赞、-1 取消、0 并发下无变更）。
	Flip(ctx context.Context, videoID, userID uint) (int64, error)
	// Create 建立点赞关系；唯一键冲突返回 ErrDuplicateLike。
	Create(ctx context.Context, videoID, userID uint) error
	// Delete 删除点赞关系，返回影响行数。
	Delete(ctx context.Context, videoID, userID uint) (int64, error)
	// Exists 判断用户是否已点赞。
	Exists(ctx context.Context, videoID, userID uint) (bool, error)
}

// CoinRepository 投币关系仓储。
type CoinRepository interface {
	// LockByVideoAndUser 以行锁读取投币关系；不存在时返回 ErrRecordNotFound。
	LockByVideoAndUser(ctx context.Context, videoID, userID uint) error
	// Create 建立投币关系；唯一键冲突返回 ErrDuplicateCoin。
	Create(ctx context.Context, videoID, userID uint, coins int64) error
	// Exists 判断用户是否已对该视频投币。
	Exists(ctx context.Context, videoID, userID uint) (bool, error)
}

// FolderRepository 收藏夹与收藏关系仓储。
type FolderRepository interface {
	// FindDefault 读取用户默认收藏夹；不存在时返回 (nil, nil)。
	FindDefault(ctx context.Context, userID uint) (*FavoriteFolder, error)
	// Create 创建收藏夹并回填 ID。
	Create(ctx context.Context, folder *FavoriteFolder) error
	// FindByIDAndUser 读取用户名下的收藏夹；不存在时返回 ErrRecordNotFound。
	FindByIDAndUser(ctx context.Context, userID, folderID uint) (*FavoriteFolder, error)
	// Update 更新收藏夹（目前仅名称）。
	Update(ctx context.Context, folder *FavoriteFolder) error
	// DeleteWithVideos 删除收藏夹：先扣减受影响视频的 favorite_count，再清关联与夹本体。
	DeleteWithVideos(ctx context.Context, userID uint, folder *FavoriteFolder) ([]uint, error)
	// LinkExists 判断某收藏夹内是否已含该视频。
	LinkExists(ctx context.Context, folderID, videoID uint) (bool, error)
	// DeleteLink 删除收藏夹与视频的关联。
	DeleteLink(ctx context.Context, folderID, videoID uint) error
	// EnsureLink 建立收藏夹与视频的关联，返回是否新建。
	EnsureLink(ctx context.Context, folderID, videoID uint) (bool, error)
	// UserFavorited 判断用户是否在任意收藏夹内收藏了该视频。
	UserFavorited(ctx context.Context, videoID, userID uint) (bool, error)
}

// HistoryRepository 播放历史仓储。
type HistoryRepository interface {
	// Upsert 写入或更新播放进度。
	Upsert(ctx context.Context, userID, videoID uint, progress, duration int) error
	// Find 读取播放进度；不存在时返回 ErrRecordNotFound。
	Find(ctx context.Context, userID, videoID uint) (*PlayHistory, error)
	// DeleteByVideoIDs 批量删除用户的历史记录。
	DeleteByVideoIDs(ctx context.Context, userID uint, videoIDs []uint) error
	// RecentVideoIDs 读取用户最近观看的视频 ID（按时间倒序，最多 limit 条）。
	RecentVideoIDs(ctx context.Context, userID uint, limit int) ([]uint, error)
	// ProgressByVideoIDs 批量读取用户在给定视频上的进度（仅返回 >0 的项）。
	ProgressByVideoIDs(ctx context.Context, userID uint, videoIDs []uint) map[uint]int
}

// PlayHistory 播放历史实体。
type PlayHistory struct {
	UserID    uint
	VideoID   uint
	Progress  int
	Duration  int
	CreatedAt time.Time
}

// PartitionRepository 分区仓储。
type PartitionRepository interface {
	// Create 创建分区并回填 ID。
	Create(ctx context.Context, p *Partition) error
	// Update 覆盖更新分区字段。
	Update(ctx context.Context, p *Partition) error
	// Delete 删除分区，返回影响行数。
	Delete(ctx context.Context, id uint) (int64, error)
	// FindByID 读取分区；不存在时返回 ErrRecordNotFound。
	FindByID(ctx context.Context, id uint) (*Partition, error)
	// List 分页列出分区。
	List(ctx context.Context, page, pageSize int) ([]Partition, int64, error)
	// ListSubmittable 列出启用且允许投稿的分区。
	ListSubmittable(ctx context.Context) ([]Partition, error)
}

// DanmuRepository 弹幕写侧仓储。
type DanmuRepository interface {
	// Create 写入弹幕并递增所属分P的弹幕数（单事务）。
	Create(ctx context.Context, record *Danmu) error
	// FindByID 读取弹幕（id/user_id/video_id/video_part_id）；不存在时返回 ErrDanmuNotFound。
	FindByID(ctx context.Context, danmuID uint) (*Danmu, error)
	// FindByIDs 批量读取弹幕（id/video_id/video_part_id）。
	FindByIDs(ctx context.Context, danmuIDs []uint) ([]Danmu, error)
	// DeleteMany 删除弹幕及其点赞（单事务）。
	DeleteMany(ctx context.Context, danmuIDs []uint) error
	// DecrPartDanmuCount 扣减分P弹幕数（不小于 0）。
	DecrPartDanmuCount(ctx context.Context, partID uint, delta int64) error
	// PartAvailability 一次查询视频与分P可用性快照。
	PartAvailability(ctx context.Context, videoID, partID uint) (DanmuPartAvailability, error)
	// ToggleLike 翻转弹幕点赞并维护 like_count；返回翻转后是否已点赞。
	ToggleLike(ctx context.Context, danmuID, userID uint) (bool, error)
	// LikeCount 读取弹幕当前点赞数。
	LikeCount(ctx context.Context, danmuID uint) (int64, error)
	// LikedSet 读取用户在给定弹幕集合上的点赞标记。
	LikedSet(ctx context.Context, userID uint, danmuIDs []uint) (map[uint]struct{}, error)
}

// ReportRepository 举报写侧仓储。
type ReportRepository interface {
	// CreateVideoReport 写入视频举报并回填 ID/时间。
	CreateVideoReport(ctx context.Context, report *VideoReport) error
	// CreateDanmuReport 写入弹幕举报并回填 ID/时间。
	CreateDanmuReport(ctx context.Context, report *DanmuReport) error
	// HandleVideoReports 批量处理待处理的视频举报，返回影响行数。
	HandleVideoReports(ctx context.Context, reportIDs []uint, status enum.ReportStatus, note string, handlerID uint, handledAt time.Time) (int64, error)
	// HandleDanmuReports 批量处理待处理的弹幕举报，返回影响行数。
	HandleDanmuReports(ctx context.Context, reportIDs []uint, status enum.ReportStatus, note string, handlerID uint, handledAt time.Time) (int64, error)
	// ReporterUsernames 读取给定举报记录的举报人用户名列表。
	ReporterUsernames(ctx context.Context, handleType ReportHandleType, reportIDs []uint) ([]string, error)
}
