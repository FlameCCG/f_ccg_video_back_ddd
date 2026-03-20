package entity

import (
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	videoerrors "f_ccg_video_back_ddd/internal/domain/video/domainerr"
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
	ApprovedAt     *time.Time
	IsScheduled    bool
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

// OwnerPreviewStatuses 作者本人可预览（非公开）的视频状态集合。
// 。
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

// maxTagNameRunes 标签名称最大字符数。
const maxTagNameRunes = 64

// ValidateTagName 校验并归一化标签名称。
func ValidateTagName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", videoerrors.ErrTagNameEmpty
	}

	if len([]rune(trimmed)) > maxTagNameRunes {
		return "", videoerrors.ErrTagNameTooLong
	}

	return trimmed, nil
}
