package entity

import (
	"strings"
	"time"

	dynamicerrors "f_ccg_video_back_ddd/internal/domain/dynamic/domainerr"
)

// MaxContentRunes 动态正文允许的最大字符数。
const MaxContentRunes = 2000

// Dynamic 动态聚合根，动态创建接口序列化由该实体派生的读模型。JSON 字段属于接口契约。
type Dynamic struct {
	ID        uint       `json:"id"`
	CreatedAt time.Time  `json:"-"`
	UpdatedAt time.Time  `json:"-"`
	UserID    uint       `json:"userId"`
	Content   string     `json:"content"`
	ImageURL  string     `json:"imageUrl"`
	LikeCount int64      `json:"likeCount"`
	IsPinned  bool       `json:"isPinned"`
	PinnedAt  *time.Time `json:"pinnedAt,omitempty"`
}

// NewDynamic 校验并构造一条待发布动态。
//
// 规则：未登录直接拒绝；
// 正文裁剪首尾空白后不得为空、不得超过 MaxContentRunes；图片 URL 同样裁剪空白。
func NewDynamic(userID uint, content, imageURL string) (*Dynamic, error) {
	if userID == 0 {
		return nil, dynamicerrors.ErrUserNotLogin
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return nil, dynamicerrors.ErrContentEmpty
	}

	if len([]rune(content)) > MaxContentRunes {
		return nil, dynamicerrors.ErrContentTooLong
	}

	return &Dynamic{
		UserID:   userID,
		Content:  content,
		ImageURL: strings.TrimSpace(imageURL),
	}, nil
}

// EnsureDeletableBy 校验删除权限：只有作者本人可删除自己的动态。
func (d *Dynamic) EnsureDeletableBy(userID uint) error {
	if d == nil {
		return dynamicerrors.ErrDynamicNotFound
	}

	if d.UserID != userID {
		return dynamicerrors.ErrDeleteForbidden
	}

	return nil
}

// EnsurePinnableBy 校验置顶权限：只有作者本人可置顶/取消置顶自己的动态。
func (d *Dynamic) EnsurePinnableBy(userID uint) error {
	if d == nil {
		return dynamicerrors.ErrDynamicNotFound
	}

	if d.UserID != userID {
		return dynamicerrors.ErrPinForbidden
	}

	return nil
}

// SetPinnedBy 由实体保护作者权限，并保持置顶状态与时间一致。
// 同作者的其它作品置顶由用例在同一事务中清除。
func (d *Dynamic) SetPinnedBy(userID uint, pinned bool, now time.Time) error {
	if err := d.EnsurePinnableBy(userID); err != nil {
		return err
	}
	d.IsPinned = pinned
	d.PinnedAt = nil
	if pinned {
		d.PinnedAt = &now
	}
	return nil
}
