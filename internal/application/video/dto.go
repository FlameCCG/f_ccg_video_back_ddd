// Package video 编排投稿、改稿、播放详情、互动、弹幕与管理。
//
// 建议按 UploadService → PublishService → domain/video → persistence/video 的顺序追一次投稿。
// PublishService/UpdateService 编排用例，domain 维护状态与改稿规则；ports.go 定义查询、缓存及跨上下文能力。
// 写侧通过领域仓储持久化，读侧直接组装 dto.go 的页面模型，这是共用存储的轻量 CQRS。
// 定时计划与视频状态同库保存；转码投递、向量更新和缓存维护有各自完成点，不属于同一个分布式事务。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package video

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/video"
)

// ============================================================================
// 详情读模型（json tag 与原 service/video_service 逐字一致；同时是 Redis meta 缓存格式）
// ============================================================================

// DetailMeta 视频详情的文本/结构化信息（有失效时间的缓存部分）。
type DetailMeta struct {
	IsScheduled bool                   `json:"isScheduled"`
	PublishAt   *time.Time             `json:"publishAt"`
	ApprovedAt  *time.Time             `json:"approvedAt"`
	ID          uint                   `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Cover       string                 `json:"cover"`
	Duration    int                    `json:"duration"`
	IsOriginal  bool                   `json:"isOriginal"`
	StorageType enum.VideoStorageType  `json:"storageType"`
	Status      enum.VideoStatus       `json:"status"`
	CreatedAt   time.Time              `json:"createdAt"`
	Partition   PartitionBrief         `json:"partition"`
	Author      AuthorBrief            `json:"author"`
	Tags        []TagBrief             `json:"tags"`
	Resources   []domain.ResourceBrief `json:"resources"`
	Parts       []PartBrief            `json:"parts,omitempty"`
}

// Detail 视频详情响应体。
type Detail struct {
	IsScheduled   bool                   `json:"isScheduled"`
	PublishAt     *time.Time             `json:"publishAt"`
	ApprovedAt    *time.Time             `json:"approvedAt"`
	ID            uint                   `json:"id"`
	Title         string                 `json:"title"`
	Description   string                 `json:"description"`
	Cover         string                 `json:"cover"`
	Duration      int                    `json:"duration"`
	Views         int64                  `json:"views"`
	Likes         int64                  `json:"likes"`
	CommentCount  int64                  `json:"commentCount"`
	CoinCount     int64                  `json:"coinCount"`
	FavoriteCount int64                  `json:"favoriteCount"`
	DanmuCount    int64                  `json:"danmuCount"`
	IsOriginal    bool                   `json:"isOriginal"`
	StorageType   enum.VideoStorageType  `json:"storageType"`
	Status        enum.VideoStatus       `json:"status"`
	CreatedAt     time.Time              `json:"createdAt"`
	Partition     PartitionBrief         `json:"partition"`
	Author        AuthorBrief            `json:"author"`
	Tags          []TagBrief             `json:"tags"`
	Resources     []domain.ResourceBrief `json:"resources"`
	Parts         []PartBrief            `json:"parts,omitempty"`
	IsLiked       bool                   `json:"isLiked"`
	IsFavorited   bool                   `json:"isFavorited"`
	IsCoined      bool                   `json:"isCoined"`
}

// PartitionBrief 分区摘要。
type PartitionBrief struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Icon          string `json:"icon"`
	IsActive      bool   `json:"isActive"`
	IsSubmittable bool   `json:"isSubmittable"`
}

// AuthorBrief 作者摘要。
type AuthorBrief struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	Avatar      string `json:"avatar"`
	Level       int    `json:"level"`
	Description string `json:"description"`
}

// TagBrief 标签摘要。
type TagBrief struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// PartBrief 分P摘要。
type PartBrief struct {
	ID         uint                   `json:"id"`
	Title      string                 `json:"title"`
	SortOrder  int                    `json:"sortOrder"`
	Duration   int                    `json:"duration"`
	DanmuCount int64                  `json:"danmuCount"`
	Resources  []domain.ResourceBrief `json:"resources"`
}

// ============================================================================
// 列表读模型
// ============================================================================

// FeedItem 首页/热门/排行/推荐的列表项。
type FeedItem struct {
	ID         uint            `json:"id"`
	Title      string          `json:"title"`
	Cover      string          `json:"cover"`
	Duration   int             `json:"duration"`
	Views      int64           `json:"views"`
	DanmuCount int64           `json:"danmuCount"`
	Author     FeedAuthorBrief `json:"author"`
	CreatedAt  time.Time       `json:"createdAt"`
}

// FeedAuthorBrief 列表项作者摘要。
type FeedAuthorBrief struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// PartitionItem 分区列表项。
type PartitionItem struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Icon          string `json:"icon"`
	SortOrder     int    `json:"sortOrder"`
	IsActive      bool   `json:"isActive"`
	IsSubmittable bool   `json:"isSubmittable"`
}

// HistoryItem 观看历史列表项。
type HistoryItem struct {
	VideoID   uint      `json:"videoId"`
	Title     string    `json:"title"`
	Cover     string    `json:"cover"`
	Duration  int       `json:"duration"`
	Progress  int       `json:"progress"`
	CreatedAt time.Time `json:"createdAt"`
	AuthorID  uint      `json:"authorId"`
	Author    string    `json:"author"`
}

// FolderWithCount 收藏夹列表项（含视频数与是否已收藏当前视频）。
type FolderWithCount struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	IsDefault   bool   `json:"isDefault"`
	VideoCount  int64  `json:"videoCount"`
	IsFavorited bool   `json:"isFavorited"`
}

// FolderVideoItem 收藏夹内视频列表项。
type FolderVideoItem struct {
	ID            uint        `json:"id"`
	Title         string      `json:"title"`
	Cover         string      `json:"cover"`
	Duration      int         `json:"duration"`
	Views         int64       `json:"views"`
	DanmuCount    int64       `json:"danmuCount"`
	FavoriteCount int64       `json:"favoriteCount"`
	FavoritedAt   time.Time   `json:"favoritedAt"`
	Author        AuthorBrief `json:"author"`
}

// UserVideoItem 用户投稿列表项。
type UserVideoItem struct {
	ID              uint      `json:"id"`
	Title           string    `json:"title"`
	Cover           string    `json:"cover"`
	Views           int64     `json:"views"`
	DanmuCount      int64     `json:"danmuCount"`
	Duration        int       `json:"duration"`
	Progress        int       `json:"progress,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	Status          int       `json:"status"`
	StatusText      string    `json:"statusText"`
	AuditStatus     int       `json:"auditStatus,omitempty"`
	AuditStatusText string    `json:"auditStatusText,omitempty"`
	AuthorID        uint      `json:"authorId,omitempty"`
	AuthorName      string    `json:"authorName,omitempty"`
}

// UserVideoRow 用户投稿列表的原始行（读侧投影，未叠加 Redis 计数与进度）。
type UserVideoRow struct {
	ID             uint
	Title          string
	Cover          string
	Views          int64
	Likes          int64
	FavoriteCount  int64
	DanmuCount     int64
	Duration       int
	CreatedAt      time.Time
	Status         enum.VideoStatus
	AuthorUsername string
	AuthorID       uint
	AuthorName     string
}

// AuthorVideoStatRow 作者作品统计聚合的原始行。
type AuthorVideoStatRow struct {
	ID    uint
	Views int64
	Likes int64
}

// UserVideoStats 聚合用户作品曝光/互动数据。
type UserVideoStats struct {
	TotalLikes int64 `json:"totalLikes"`
	TotalViews int64 `json:"totalViews"`
}

// ============================================================================
// 弹幕读模型
// ============================================================================

// DanmuDTO 弹幕响应体（同时是 Redis 最近弹幕缓存格式）。
type DanmuDTO struct {
	ID          uint      `json:"id"`
	VideoID     uint      `json:"videoId"`
	VideoPartID uint      `json:"videoPartId"`
	UserID      uint      `json:"userId"`
	Content     string    `json:"content"`
	TimeOffset  int       `json:"timeOffset"`
	Color       string    `json:"color"`
	FontSize    int8      `json:"fontSize"`
	Position    int8      `json:"position"`
	LikeCount   int64     `json:"likeCount"`
	IsLiked     bool      `json:"isLiked"`
	CreatedAt   time.Time `json:"createdAt"`
}

// DanmuRoomItem 房间弹幕列表项。
type DanmuRoomItem struct {
	ID          uint      `json:"id"`
	VideoID     uint      `json:"videoId"`
	VideoPartID uint      `json:"videoPartId"`
	UserID      uint      `json:"userId"`
	Content     string    `json:"content"`
	TimeOffset  int       `json:"timeOffset"`
	Color       string    `json:"color"`
	FontSize    int8      `json:"fontSize"`
	Position    int8      `json:"position"`
	LikeCount   int64     `json:"likeCount"`
	IsLiked     bool      `json:"isLiked"`
	CreatedAt   time.Time `json:"createdAt"`
}

// DanmuLikeResult 弹幕点赞结果。
type DanmuLikeResult struct {
	LikeCount int64 `json:"likeCount"`
	IsLiked   bool  `json:"isLiked"`
}

// AdminDanmuItem 管理端弹幕列表项。
type AdminDanmuItem struct {
	ID          uint      `json:"id"`
	UserID      uint      `json:"userId"`
	Username    string    `json:"username"`
	Avatar      string    `json:"avatar"`
	VideoID     uint      `json:"videoId"`
	VideoTitle  string    `json:"videoTitle,omitempty"`
	VideoCover  string    `json:"videoCover,omitempty"`
	VideoPartID uint      `json:"videoPartId"`
	Content     string    `json:"content"`
	TimeOffset  int       `json:"timeOffset"`
	Color       string    `json:"color"`
	FontSize    int8      `json:"fontSize"`
	Position    int8      `json:"position"`
	LikeCount   int64     `json:"likeCount"`
	CreatedAt   time.Time `json:"createdAt"`
}

// CreatorDanmuItem 创作中心弹幕列表项。
type CreatorDanmuItem struct {
	ID          uint      `json:"id"`
	UserID      uint      `json:"userId"`
	Username    string    `json:"username"`
	Avatar      string    `json:"avatar"`
	VideoID     uint      `json:"videoId"`
	VideoTitle  string    `json:"videoTitle,omitempty"`
	VideoCover  string    `json:"videoCover,omitempty"`
	VideoPartID uint      `json:"videoPartId"`
	Content     string    `json:"content"`
	TimeOffset  int       `json:"timeOffset"`
	Color       string    `json:"color"`
	FontSize    int8      `json:"fontSize"`
	Position    int8      `json:"position"`
	LikeCount   int64     `json:"likeCount"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ============================================================================
// 管理端读模型
// ============================================================================

// AdminVideoAuthor 管理端视频列表的作者信息。
type AdminVideoAuthor struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// AdminVideoItem 管理端视频列表项。
type AdminVideoItem struct {
	IsScheduled   bool             `json:"isScheduled"`
	PublishAt     *time.Time       `json:"publishAt"`
	ApprovedAt    *time.Time       `json:"approvedAt"`
	ID            uint             `json:"id"`
	Title         string           `json:"title"`
	Cover         string           `json:"cover"`
	Duration      int              `json:"duration"`
	Views         int64            `json:"views"`
	Likes         int64            `json:"likes"`
	CommentCount  int64            `json:"commentCount"`
	CoinCount     int64            `json:"coinCount"`
	FavoriteCount int64            `json:"favoriteCount"`
	DanmuCount    int64            `json:"danmuCount"`
	IsOriginal    bool             `json:"isOriginal"`
	StorageType   string           `json:"storageType"`
	Status        enum.VideoStatus `json:"status"`
	PartitionID   uint             `json:"partitionId"`
	PartitionName string           `json:"partitionName"`
	Author        AdminVideoAuthor `json:"author"`
	CreatedAt     time.Time        `json:"createdAt"`
}

// AdminVideoReportItem 管理端视频举报列表项。
type AdminVideoReportItem struct {
	ID               uint       `json:"id"`
	ReporterUsername string     `json:"reporterUsername"`
	VideoID          uint       `json:"videoId"`
	VideoTitle       string     `json:"videoTitle"`
	VideoCover       string     `json:"videoCover"`
	Reason           string     `json:"reason"`
	Detail           string     `json:"detail"`
	ImageUrls        []string   `json:"imageUrls"`
	Status           int        `json:"status"`
	HandleNote       string     `json:"handleNote"`
	HandledBy        uint       `json:"handledBy"`
	HandledAt        *time.Time `json:"handledAt"`
	CreatedAt        time.Time  `json:"createdAt"`
}

// AdminDanmuReportItem 管理端弹幕举报列表项。
type AdminDanmuReportItem struct {
	ID               uint       `json:"id"`
	ReporterUsername string     `json:"reporterUsername"`
	VideoID          uint       `json:"videoId"`
	VideoTitle       string     `json:"videoTitle"`
	VideoCover       string     `json:"videoCover"`
	DanmuID          uint       `json:"danmuId"`
	DanmuContent     string     `json:"danmuContent"`
	Reason           string     `json:"reason"`
	Detail           string     `json:"detail"`
	Status           int        `json:"status"`
	HandleNote       string     `json:"handleNote"`
	HandledBy        uint       `json:"handledBy"`
	HandledAt        *time.Time `json:"handledAt"`
	CreatedAt        time.Time  `json:"createdAt"`
}

// ============================================================================
// 互动结果
// ============================================================================

// LikeResult 点赞结果。
type LikeResult struct {
	Likes int64 `json:"likes"`
}

// CoinResult 投币结果。
type CoinResult struct {
	CoinCount int64 `json:"coinCount"`
}

// FavoriteResult 收藏结果。
type FavoriteResult struct {
	FavoriteCount int64 `json:"favoriteCount"`
}

// ViewResult 播放量结果。
type ViewResult struct {
	Views int64 `json:"views"`
}

// TripleResult 一键三连结果。
type TripleResult struct {
	Likes         int64 `json:"likes"`
	CoinCount     int64 `json:"coinCount"`
	FavoriteCount int64 `json:"favoriteCount"`
	IsLiked       bool  `json:"isLiked"`
	IsFavorited   bool  `json:"isFavorited"`
	IsCoined      bool  `json:"isCoined"`
}

// ============================================================================
// 查询参数
// ============================================================================

// SendDanmuParams 发送弹幕参数。
type SendDanmuParams struct {
	VideoID     uint
	VideoPartID uint
	Content     string
	TimeOffset  int
	Color       string
	FontSize    int8
	Position    int8
}

// DanmuHistoryParams 历史弹幕查询参数。
type DanmuHistoryParams struct {
	VideoID     uint
	VideoPartID uint
	UserID      uint
	Start       int
	End         int
	Limit       int
}

// DanmuMonthDates 某月内有弹幕的发送时刻。
//
// Times 是库内 Asia/Shanghai 的绝对时间。浏览器再用本地时区把它折成日历日。
type DanmuMonthDates struct {
	Month string      `json:"month"`
	Times []time.Time `json:"times"`
}

// DanmuRoomListParams 房间弹幕列表查询参数。
type DanmuRoomListParams struct {
	VideoID      uint
	VideoPartID  uint
	Page         int
	PageSize     int
	Date         string // YYYY-MM-DD, optional
	UserID       uint   // optional (0 means anonymous)
	Start        int    // 播放时间起点（毫秒，含）
	End          int    // 播放时间终点（毫秒，不含）
	UseTimeRange bool   // true 时按 [Start, End) 查询播放分段
}

// DanmuRoomQueryParams 房间弹幕的读侧查询参数（已完成归一化）。
type DanmuRoomQueryParams struct {
	VideoID      uint
	VideoPartID  uint
	Page         int
	PageSize     int
	UseTimeRange bool
	Start        int
	End          int
	// DateStart/DateEnd 为 Date 解析结果；HasDate 为 false 时忽略。
	HasDate   bool
	DateStart time.Time
	DateEnd   time.Time
}

// AdminDanmuListParams 管理端弹幕列表查询参数。
type AdminDanmuListParams struct {
	UserID   uint
	Username string
	Keyword  string
	Page     int
	PageSize int
}

// AdminVideoListParams 管理端视频列表查询参数。
type AdminVideoListParams struct {
	IsScheduled *bool
	UserID      uint
	Keyword     string
	Status      int
	PartitionID uint
	Sort        string // latest/oldest/views/likes/duration
	Page        int
	PageSize    int
	// AuthorUsername 由应用层按 UserID 解析后填充（读侧投影）。
	AuthorUsername string
}

// AdminVideoReviewParams 管理端审核参数。
type AdminVideoReviewParams struct {
	VideoIDs []uint
	Status   enum.VideoStatus
}

// UpdateVideoOpts 改稿参数。
type UpdateVideoOpts struct {
	VideoID     uint
	Title       *string
	Description *string
	PartitionID *uint
	TagIDs      []uint
	IsOriginal  *bool
	IsPrivate   *bool
	CoverURL    *string
	PublishTime *time.Time
}

// UpdatedVideo 改稿后的视频快照。
//
// 原实现直接序列化 *mysqlEntity.Video（Author 与 VideoPartition 虽带 omitempty
// 但结构体字段不会被省略，因此响应里始终有 author / category 两个对象）。
// 该 DTO 逐字段复刻其序列化形态，保证响应 JSON 1:1。
type UpdatedVideo struct {
	ID             uint                  `json:"id"`
	Title          string                `json:"title"`
	Description    string                `json:"description"`
	Cover          string                `json:"cover"`
	Duration       int                   `json:"duration"`
	AuthorUsername string                `json:"authorUsername"`
	PartitionID    uint                  `json:"partitionID"`
	Views          int64                 `json:"views"`
	Likes          int64                 `json:"likes"`
	CommentCount   int64                 `json:"commentCount"`
	CoinCount      int64                 `json:"coinCount"`
	FavoriteCount  int64                 `json:"favoriteCount"`
	DanmuCount     int64                 `json:"danmuCount"`
	IsPinned       bool                  `json:"isPinned"`
	PinnedAt       *time.Time            `json:"pinnedAt,omitempty"`
	Tags           []UpdatedVideoTag     `json:"tags"`
	IsOriginal     bool                  `json:"isOriginal"`
	StorageType    enum.VideoStorageType `json:"storageType"`
	Status         enum.VideoStatus      `json:"status"`
	Author         UpdatedVideoAuthor    `json:"author"`
	Partition      UpdatedVideoPartition `json:"category"`
}

// UpdatedVideoTag 改稿响应中的标签（形态等同 po.Tag 序列化结果）。
type UpdatedVideoTag struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// UpdatedVideoAuthor 改稿响应中的作者对象。
//
// 原实现从不预载 Author，因此这里始终是零值；字段与 po.User 的序列化形态一致。
type UpdatedVideoAuthor struct {
	ID             uint            `json:"id"`
	Username       string          `json:"username"`
	Email          string          `json:"email"`
	Avatar         string          `json:"avatar"`
	Description    string          `json:"description"`
	Gender         int             `json:"gender"`
	Birthday       *time.Time      `json:"birthday"`
	Level          int             `json:"level"`
	OpenID         string          `json:"openID"`
	RegisterSource string          `json:"registerSource"`
	Exp            int64           `json:"exp"`
	CoinCount      float64         `json:"coinCount"`
	Status         enum.UserStatus `json:"status"`
	CurrentRoleID  uint            `json:"currentRoleID"`
	RegisterIP     string          `json:"registerIP"`
	LastLoginIP    string          `json:"lastLoginIP"`
	RegisterAddr   string          `json:"registerAddr"`
	LastLoginAddr  string          `json:"lastLoginAddr"`
}

// UpdatedVideoPartition 改稿响应中的分区对象（形态等同 po.VideoPartition 序列化结果）。
type UpdatedVideoPartition struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Icon          string `json:"icon"`
	SortOrder     int    `json:"sortOrder"`
	IsActive      bool   `json:"isActive"`
	IsSubmittable bool   `json:"isSubmittable"`
}

// FolderDTO 收藏夹响应体（形态等同 po.FavoriteFolder 序列化结果）。
type FolderDTO struct {
	ID        uint   `json:"id"`
	UserID    uint   `json:"userId"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

// ReportDTO 视频举报响应体（形态等同 po.Report 序列化结果）。
type ReportDTO struct {
	ID               uint              `json:"id"`
	ReporterUsername string            `json:"reporterUsername"`
	VideoID          uint              `json:"videoId"`
	Reason           string            `json:"reason"`
	Detail           string            `json:"detail"`
	ImageUrls        []string          `json:"imageUrls"`
	Status           enum.ReportStatus `json:"status"`
	HandleNote       string            `json:"handleNote"`
	HandledBy        uint              `json:"handledBy"`
	HandledAt        *time.Time        `json:"handledAt"`
}

// DanmuReportDTO 弹幕举报响应体（形态等同 po.DanmuReport 序列化结果）。
type DanmuReportDTO struct {
	ID               uint              `json:"id"`
	ReporterUsername string            `json:"reporterUsername"`
	VideoID          uint              `json:"videoId"`
	DanmuID          uint              `json:"danmuId"`
	Reason           string            `json:"reason"`
	Detail           string            `json:"detail"`
	Status           enum.ReportStatus `json:"status"`
	HandleNote       string            `json:"handleNote"`
	HandledBy        uint              `json:"handledBy"`
	HandledAt        *time.Time        `json:"handledAt"`
}

// UploadStatus 分片上传状态。
type UploadStatus struct {
	FileHash       string
	Completed      bool
	FilePath       string
	UploadedChunks []string
}

// TranscodeProgressItem 转码进度项。
//
// json tag 与原 service/mq_service.TranscodeProgress 逐字一致：
// 该结构会直接写入 SSE 的 data 字段与轮询响应，属外部契约。
type TranscodeProgressItem struct {
	VideoID      uint                        `json:"videoId"`
	PartID       uint                        `json:"partId,omitempty"`
	Status       string                      `json:"status"`
	Percent      float64                     `json:"percent"`
	Stage        string                      `json:"stage"`
	StagePercent float64                     `json:"stagePercent"`
	Message      string                      `json:"message,omitempty"`
	DurationSec  int                         `json:"durationSec,omitempty"`
	OutTimeSec   float64                     `json:"outTimeSec,omitempty"`
	Error        string                      `json:"error,omitempty"`
	UpdatedAt    time.Time                   `json:"updatedAt"`
	StartedAt    time.Time                   `json:"startedAt"`
	Pipelines    []TranscodePipelineProgress `json:"pipelines,omitempty"`
}

// TranscodePipelineProgress 单个 FFmpeg 管线的进度。
type TranscodePipelineProgress struct {
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Percent     float64   `json:"percent"`
	Message     string    `json:"message,omitempty"`
	Error       string    `json:"error,omitempty"`
	Resolutions []uint16  `json:"resolutions,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// PublishVideoCommand 投稿命令（handler 校验绑定后交由应用层执行）。
type PublishVideoCommand struct {
	AuthorID       uint
	Title          string
	Description    string
	PartitionID    uint
	TagIDs         []uint
	IsOriginal     bool
	IsPrivate      bool
	CoverURL       string
	FilePath       string
	FileName       string
	FileHash       string
	Parts          []PublishPartInput
	PublishTime    time.Time
	AuthorUsername string
}

// PublishPartInput 投稿分P输入。
type PublishPartInput struct {
	Title    string
	FilePath string
	FileName string
	FileHash string
}

// PublishVideoResult 投稿结果。
type PublishVideoResult struct {
	VideoID     uint
	Status      enum.VideoStatus
	PublishTime time.Time
	Cover       string
}
