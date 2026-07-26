package video

import (
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Video 视频实体（视频上下文的聚合根）。
//
// 仅保留写侧业务规则需要的字段；接口响应所需的富嵌套结构走读侧读模型
// （见 architecture.md 第 4 节与 application/video 的 DTO）。
type Video struct {
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
}

// IsPublished 判断视频是否处于已发布（公开可见）状态。
func (v *Video) IsPublished() bool {
	if v == nil {
		return false
	}

	return v.Status == enum.VideoStatusPublished
}

// IsDeleted 判断视频是否已删除。
func (v *Video) IsDeleted() bool {
	if v == nil {
		return false
	}

	return v.Status == enum.VideoStatusDeleted
}

// OwnedBy 判断视频是否属于指定用户名的作者。
func (v *Video) OwnedBy(username string) bool {
	if v == nil || username == "" {
		return false
	}

	return v.AuthorUsername == username
}

// Stats 视频数字类统计（读写共用的值对象）。
//
// json tag 与原 service/video_service.VideoStats 逐字一致：
// Redis 缓存中的 stats payload 会按该结构序列化，属于外部契约。
type Stats struct {
	Views         int64 `json:"views"`
	Likes         int64 `json:"likes"`
	CommentCount  int64 `json:"commentCount"`
	CoinCount     int64 `json:"coinCount"`
	FavoriteCount int64 `json:"favoriteCount"`
	DanmuCount    int64 `json:"danmuCount"`
}

// StatsDelta 视频统计增量。
type StatsDelta struct {
	Views         int64
	Likes         int64
	CommentCount  int64
	CoinCount     int64
	FavoriteCount int64
	DanmuCount    int64
}

// IsZero 判断增量是否为空（无需任何写入）。
func (d StatsDelta) IsZero() bool {
	return d.Views == 0 &&
		d.Likes == 0 &&
		d.CommentCount == 0 &&
		d.CoinCount == 0 &&
		d.FavoriteCount == 0 &&
		d.DanmuCount == 0
}

// UserFlags 当前用户对视频的互动标记。
type UserFlags struct {
	IsLiked     bool
	IsFavorited bool
	IsCoined    bool
}

// OwnerPreviewStatuses 作者本人可预览（非公开）的视频状态集合。
//
// 与原 loadOwnerPreviewVideoWithAssociations 的 status IN 列表逐项一致。
func OwnerPreviewStatuses() []enum.VideoStatus {
	return []enum.VideoStatus{
		enum.VideoStatusPrivate,
		enum.VideoStatusReview,
		enum.VideoStatusScheduled,
	}
}

// OwnerVisibleStatuses 作者本人在个人页可见的视频状态集合（默认过滤已删除）。
func OwnerVisibleStatuses() []enum.VideoStatus {
	return []enum.VideoStatus{
		enum.VideoStatusPublished,
		enum.VideoStatusPrivate,
		enum.VideoStatusReview,
		enum.VideoStatusScheduled,
	}
}

// Tag 标签实体。
type Tag struct {
	ID   uint
	Name string
}

// maxTagNameRunes 标签名称最大字符数（与原 TagService.CreateOrGet 一致）。
const maxTagNameRunes = 64

// ValidateTagName 校验并归一化标签名称。
func ValidateTagName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ErrTagNameEmpty
	}

	if len([]rune(trimmed)) > maxTagNameRunes {
		return "", ErrTagNameTooLong
	}

	return trimmed, nil
}
