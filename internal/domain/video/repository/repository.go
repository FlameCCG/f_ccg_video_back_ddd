package repository

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	videoentity "f_ccg_video_back_ddd/internal/domain/video/entity"
)

// VideoRepository 稿件加载、字段级改稿和生命周期状态仓储。
// ForUpdate 方法必须使用调用方事务 context，不提供全实体 Save。
type VideoRepository interface {
	// FindByID 按主键读取视频；不存在时返回 (nil, nil)。
	FindByID(ctx context.Context, videoID uint) (*videoentity.Video, error)
	// FindByIDForUpdate 在调用方事务中锁定视频，供审核和到期发布协调状态与计划。
	FindByIDForUpdate(ctx context.Context, videoID uint) (*videoentity.Video, error)
	// FindByIDsForUpdate 按 ID 升序锁定一批视频；不存在的 ID 忽略，用于批量生命周期操作。
	FindByIDsForUpdate(ctx context.Context, videoIDs []uint) ([]*videoentity.Video, error)
	// FindByIDAndAuthorForUpdate 在调用方事务内锁定视频，直到本次改稿提交或回滚。
	// 不存在时返回 ErrRecordNotFound；调用方必须传入 tx.Manager 的事务 context。
	FindByIDAndAuthorForUpdate(ctx context.Context, videoID uint, authorUsername string) (*videoentity.Video, error)
	// UpdateStatus 无条件更新视频状态。
	UpdateStatus(ctx context.Context, videoID uint, status enum.VideoStatus) error
	// UpdateStatusFrom 仅当当前状态等于 from 时更新为 to，返回影响行数。
	UpdateStatusFrom(ctx context.Context, videoID uint, from, to enum.VideoStatus) (int64, error)
	// Approve 在调用方事务中将待审视频置为目标状态并记录实际审核通过时间。
	Approve(ctx context.Context, videoID uint, status enum.VideoStatus, approvedAt time.Time) (int64, error)
	// UpdateStatusBatch 批量更新视频状态，返回影响行数。
	UpdateStatusBatch(ctx context.Context, videoIDs []uint, status enum.VideoStatus) (int64, error)
	// UpdateStatusBatchFrom 批量条件更新视频状态，返回影响行数。
	UpdateStatusBatchFrom(ctx context.Context, videoIDs []uint, from, to enum.VideoStatus) (int64, error)
	// IDsByStatus 过滤出指定状态的视频 ID。
	IDsByStatus(ctx context.Context, videoIDs []uint, status enum.VideoStatus) ([]uint, error)
	// SaveRevision 保存已锁定、经 Video.Revise 校验的改稿快照；不覆盖计数与媒体资源。
	SaveRevision(ctx context.Context, video *videoentity.Video) error
}

// HardDeleteResult 物理删除的各表影响行数。
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
	FindDefault(ctx context.Context, userID uint) (*videoentity.FavoriteFolder, error)
	// Create 创建收藏夹并回填 ID。
	Create(ctx context.Context, folder *videoentity.FavoriteFolder) error
	// FindByIDAndUser 读取用户名下的收藏夹；不存在时返回 ErrRecordNotFound。
	FindByIDAndUser(ctx context.Context, userID, folderID uint) (*videoentity.FavoriteFolder, error)
	// Update 更新收藏夹（目前仅名称）。
	Update(ctx context.Context, folder *videoentity.FavoriteFolder) error
	// DeleteWithVideos 删除收藏夹：先扣减受影响视频的 favorite_count，再清关联与夹本体。
	DeleteWithVideos(ctx context.Context, userID uint, folder *videoentity.FavoriteFolder) ([]uint, error)
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
	Create(ctx context.Context, p *videoentity.Partition) error
	// Update 覆盖更新分区字段。
	Update(ctx context.Context, p *videoentity.Partition) error
	// Delete 删除分区，返回影响行数。
	Delete(ctx context.Context, id uint) (int64, error)
	// FindByID 读取分区；不存在时返回 ErrRecordNotFound。
	FindByID(ctx context.Context, id uint) (*videoentity.Partition, error)
	// List 分页列出分区。
	List(ctx context.Context, page, pageSize int) ([]videoentity.Partition, int64, error)
	// ListSubmittable 列出启用且允许投稿的分区。
	ListSubmittable(ctx context.Context) ([]videoentity.Partition, error)
}

// DanmuRepository 弹幕写侧仓储。
type DanmuRepository interface {
	// Create 写入弹幕并递增所属分P的弹幕数（单事务）。
	Create(ctx context.Context, record *videoentity.Danmu) error
	// FindByID 读取弹幕（id/user_id/video_id/video_part_id）；不存在时返回 ErrDanmuNotFound。
	FindByID(ctx context.Context, danmuID uint) (*videoentity.Danmu, error)
	// FindByIDs 批量读取弹幕（id/video_id/video_part_id）。
	FindByIDs(ctx context.Context, danmuIDs []uint) ([]videoentity.Danmu, error)
	// DeleteMany 删除弹幕及其点赞（单事务）。
	DeleteMany(ctx context.Context, danmuIDs []uint) error
	// DecrPartDanmuCount 扣减分P弹幕数（不小于 0）。
	DecrPartDanmuCount(ctx context.Context, partID uint, delta int64) error
	// PartAvailability 一次查询视频与分P可用性快照。
	PartAvailability(ctx context.Context, videoID, partID uint) (videoentity.DanmuPartAvailability, error)
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
	CreateVideoReport(ctx context.Context, report *videoentity.VideoReport) error
	// CreateDanmuReport 写入弹幕举报并回填 ID/时间。
	CreateDanmuReport(ctx context.Context, report *videoentity.DanmuReport) error
	// HandleVideoReports 批量处理待处理的视频举报，返回影响行数。
	HandleVideoReports(ctx context.Context, reportIDs []uint, status enum.ReportStatus, note string, handlerID uint, handledAt time.Time) (int64, error)
	// HandleDanmuReports 批量处理待处理的弹幕举报，返回影响行数。
	HandleDanmuReports(ctx context.Context, reportIDs []uint, status enum.ReportStatus, note string, handlerID uint, handledAt time.Time) (int64, error)
	// ReporterUsernames 读取给定举报记录的举报人用户名列表。
	ReporterUsernames(ctx context.Context, handleType videoentity.ReportHandleType, reportIDs []uint) ([]string, error)
}
