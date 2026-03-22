package video

import (
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Video 视频实体（视频上下文的聚合根）。
//
// 仅保留写侧业务规则需要的字段；接口响应所需的富嵌套结构走读侧读模型。
type Video struct {
	id             uint
	createdAt      time.Time
	updatedAt      time.Time
	title          string
	description    string
	cover          string
	duration       int
	authorUsername string
	partitionID    uint
	views          int64
	likes          int64
	commentCount   int64
	coinCount      int64
	favoriteCount  int64
	danmuCount     int64
	isOriginal     bool
	storageType    enum.VideoStorageType
	status         enum.VideoStatus
	approvedAt     *time.Time
	isScheduled    bool
}

// VideoSnapshot 是视频的持久化快照。字段公开，供仓储重建历史数据。
type VideoSnapshot struct {
	ID             uint
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Title          string
	Description    string
	Cover          string
	Duration       int
	AuthorUsername string
	PartitionID    uint
	Views          int64
	Likes          int64
	CommentCount   int64
	CoinCount      int64
	FavoriteCount  int64
	DanmuCount     int64
	IsOriginal     bool
	StorageType    enum.VideoStorageType
	Status         enum.VideoStatus
	ApprovedAt     *time.Time
	IsScheduled    bool
}

// RestoreVideo 从持久化快照恢复视频，不重新执行投稿校验。
func RestoreVideo(snapshot VideoSnapshot) *Video {
	return &Video{
		id:             snapshot.ID,
		createdAt:      snapshot.CreatedAt,
		updatedAt:      snapshot.UpdatedAt,
		title:          snapshot.Title,
		description:    snapshot.Description,
		cover:          snapshot.Cover,
		duration:       snapshot.Duration,
		authorUsername: snapshot.AuthorUsername,
		partitionID:    snapshot.PartitionID,
		views:          snapshot.Views,
		likes:          snapshot.Likes,
		commentCount:   snapshot.CommentCount,
		coinCount:      snapshot.CoinCount,
		favoriteCount:  snapshot.FavoriteCount,
		danmuCount:     snapshot.DanmuCount,
		isOriginal:     snapshot.IsOriginal,
		storageType:    snapshot.StorageType,
		status:         snapshot.Status,
		approvedAt:     (Video{}).cloneTime(snapshot.ApprovedAt),
		isScheduled:    snapshot.IsScheduled,
	}
}

// Snapshot 返回视频快照。审核时间是副本，调用方改它不会影响实体。
func (v *Video) Snapshot() VideoSnapshot {
	if v == nil {
		return VideoSnapshot{}
	}

	return VideoSnapshot{
		ID:             v.id,
		CreatedAt:      v.createdAt,
		UpdatedAt:      v.updatedAt,
		Title:          v.title,
		Description:    v.description,
		Cover:          v.cover,
		Duration:       v.duration,
		AuthorUsername: v.authorUsername,
		PartitionID:    v.partitionID,
		Views:          v.views,
		Likes:          v.likes,
		CommentCount:   v.commentCount,
		CoinCount:      v.coinCount,
		FavoriteCount:  v.favoriteCount,
		DanmuCount:     v.danmuCount,
		IsOriginal:     v.isOriginal,
		StorageType:    v.storageType,
		Status:         v.status,
		ApprovedAt:     (Video{}).cloneTime(v.approvedAt),
		IsScheduled:    v.isScheduled,
	}
}

func (v *Video) ID() uint {
	if v == nil {
		return 0
	}

	return v.id
}

func (v *Video) CreatedAt() time.Time {
	if v == nil {
		return time.Time{}
	}

	return v.createdAt
}

func (v *Video) UpdatedAt() time.Time {
	if v == nil {
		return time.Time{}
	}

	return v.updatedAt
}

func (v *Video) Title() string {
	if v == nil {
		return ""
	}

	return v.title
}

func (v *Video) Description() string {
	if v == nil {
		return ""
	}

	return v.description
}

func (v *Video) Cover() string {
	if v == nil {
		return ""
	}

	return v.cover
}

func (v *Video) Duration() int {
	if v == nil {
		return 0
	}

	return v.duration
}

func (v *Video) AuthorUsername() string {
	if v == nil {
		return ""
	}

	return v.authorUsername
}

func (v *Video) PartitionID() uint {
	if v == nil {
		return 0
	}

	return v.partitionID
}

func (v *Video) Views() int64 {
	if v == nil {
		return 0
	}

	return v.views
}

func (v *Video) Likes() int64 {
	if v == nil {
		return 0
	}

	return v.likes
}

func (v *Video) CommentCount() int64 {
	if v == nil {
		return 0
	}

	return v.commentCount
}

func (v *Video) CoinCount() int64 {
	if v == nil {
		return 0
	}

	return v.coinCount
}

func (v *Video) FavoriteCount() int64 {
	if v == nil {
		return 0
	}

	return v.favoriteCount
}

func (v *Video) DanmuCount() int64 {
	if v == nil {
		return 0
	}

	return v.danmuCount
}

func (v *Video) IsOriginal() bool {
	return v != nil && v.isOriginal
}

func (v *Video) StorageType() enum.VideoStorageType {
	if v == nil {
		return ""
	}

	return v.storageType
}

func (v *Video) Status() enum.VideoStatus {
	if v == nil {
		return 0
	}

	return v.status
}

// ApprovedAt 返回审核通过时间的副本。
func (v *Video) ApprovedAt() *time.Time {
	if v == nil {
		return nil
	}

	return (Video{}).cloneTime(v.approvedAt)
}

func (v *Video) IsScheduled() bool {
	return v != nil && v.isScheduled
}

func (Video) cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	copied := *value

	return &copied
}

// IsPublished 判断视频是否处于已发布（公开可见）状态。
func (v *Video) IsPublished() bool {
	if v == nil {
		return false
	}

	return v.status == enum.VideoStatusPublished
}

// IsDeleted 判断视频是否已删除。
func (v *Video) IsDeleted() bool {
	if v == nil {
		return false
	}

	return v.status == enum.VideoStatusDeleted
}

// OwnedBy 判断视频是否属于指定用户名的作者。
func (v *Video) OwnedBy(username string) bool {
	if v == nil || username == "" {
		return false
	}

	return v.authorUsername == username
}

// OwnerPreviewStatuses 作者本人可预览（非公开）的视频状态集合。
// 。
func (Video) OwnerPreviewStatuses() []enum.VideoStatus {
	return []enum.VideoStatus{
		enum.VideoStatusPrivate,
		enum.VideoStatusReview,
		enum.VideoStatusScheduled,
	}
}

// OwnerVisibleStatuses 作者本人在个人页可见的视频状态集合（默认过滤已删除）。
func (Video) OwnerVisibleStatuses() []enum.VideoStatus {
	return []enum.VideoStatus{
		enum.VideoStatusPublished,
		enum.VideoStatusPrivate,
		enum.VideoStatusReview,
		enum.VideoStatusScheduled,
	}
}

// DueScanStatuses 是定时发布轮询要扫描的状态。
// 开启审核时不扫描 Review，避免轮询替代人工审核。
func (Video) DueScanStatuses(reviewEnabled bool) []enum.VideoStatus {
	statuses := []enum.VideoStatus{enum.VideoStatusScheduled}
	if !reviewEnabled {
		statuses = append(statuses, enum.VideoStatusReview)
	}

	return statuses
}

// MapAuditStatus 把视频状态映射为用户主页的审核态编码与文案。
//
// 仅在「本人查看」时展示；文案。
func (Video) MapAuditStatus(status enum.VideoStatus) (int, string) {
	switch status {
	case enum.VideoStatusPublished:
		return UserVideoAuditPublished, "已发布"
	case enum.VideoStatusPrivate:
		return UserVideoAuditPrivate, "私密"
	case enum.VideoStatusReview:
		return UserVideoAuditReviewing, "审核中"
	case enum.VideoStatusScheduled:
		return UserVideoAuditScheduled, "定时发布"
	default:
		return 0, ""
	}
}

// MapStatusText 把视频状态映射为用户主页展示文案。
//
// 文案。
func (Video) MapStatusText(status enum.VideoStatus) string {
	switch status {
	case enum.VideoStatusPublished:
		return "已发布"
	case enum.VideoStatusPrivate:
		return "私密"
	case enum.VideoStatusDeleted:
		return "已删除"
	case enum.VideoStatusReview:
		return "审核中"
	case enum.VideoStatusScheduled:
		return "定时发布"
	default:
		return ""
	}
}

// Tag 标签实体。
type Tag struct {
	ID   uint
	Name string
}

// maxTagNameRunes 标签名称最大字符数。
const maxTagNameRunes = 64

// NormalizeName 校验并归一化标签名称。
func (Tag) NormalizeName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ErrTagNameEmpty
	}

	if len([]rune(trimmed)) > maxTagNameRunes {
		return "", ErrTagNameTooLong
	}

	return trimmed, nil
}
