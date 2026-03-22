package dynamic

import (
	"strings"
	"time"
)

// MaxContentRunes 动态正文允许的最大字符数。
const MaxContentRunes = 2000

// Dynamic 动态聚合根。
//
// 发布与列表的 HTTP 响应使用应用层 DynamicItem，不直接序列化本实体。
type Dynamic struct {
	id        uint
	createdAt time.Time
	updatedAt time.Time
	userID    uint
	content   string
	imageURL  string
	likeCount int64
	isPinned  bool
	pinnedAt  *time.Time
}

// DynamicSnapshot 动态的持久化快照。字段公开，重建时不重复执行创建校验。
type DynamicSnapshot struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time
	UserID    uint
	Content   string
	ImageURL  string
	LikeCount int64
	IsPinned  bool
	PinnedAt  *time.Time
}

// NewDynamic 校验并构造一条待发布动态。
//
// 规则：未登录直接拒绝；
// 正文裁剪首尾空白后不得为空、不得超过 MaxContentRunes；图片 URL 同样裁剪空白。
func NewDynamic(userID uint, content, imageURL string) (*Dynamic, error) {
	if userID == 0 {
		return nil, ErrUserNotLogin
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrContentEmpty
	}

	if len([]rune(content)) > MaxContentRunes {
		return nil, ErrContentTooLong
	}

	return &Dynamic{
		userID:   userID,
		content:  content,
		imageURL: strings.TrimSpace(imageURL),
	}, nil
}

// RestoreDynamic 从持久化快照恢复动态，不重新执行创建校验。
func RestoreDynamic(snapshot DynamicSnapshot) *Dynamic {
	d := &Dynamic{
		id:        snapshot.ID,
		createdAt: snapshot.CreatedAt,
		updatedAt: snapshot.UpdatedAt,
		userID:    snapshot.UserID,
		content:   snapshot.Content,
		imageURL:  snapshot.ImageURL,
		likeCount: snapshot.LikeCount,
		isPinned:  snapshot.IsPinned,
	}
	d.pinnedAt = d.cloneTime(snapshot.PinnedAt)

	return d
}

func (d *Dynamic) ID() uint {
	if d == nil {
		return 0
	}

	return d.id
}

func (d *Dynamic) CreatedAt() time.Time {
	if d == nil {
		return time.Time{}
	}

	return d.createdAt
}

func (d *Dynamic) UpdatedAt() time.Time {
	if d == nil {
		return time.Time{}
	}

	return d.updatedAt
}

func (d *Dynamic) UserID() uint {
	if d == nil {
		return 0
	}

	return d.userID
}

func (d *Dynamic) Content() string {
	if d == nil {
		return ""
	}

	return d.content
}

func (d *Dynamic) ImageURL() string {
	if d == nil {
		return ""
	}

	return d.imageURL
}

func (d *Dynamic) LikeCount() int64 {
	if d == nil {
		return 0
	}

	return d.likeCount
}

func (d *Dynamic) IsPinned() bool {
	if d == nil {
		return false
	}

	return d.isPinned
}

// PinnedAt 返回置顶时间的副本，调用方修改返回值不会影响动态。
func (d *Dynamic) PinnedAt() *time.Time {
	if d == nil {
		return nil
	}

	return d.cloneTime(d.pinnedAt)
}

// Snapshot 返回当前状态的副本。修改返回值不会影响动态。
func (d *Dynamic) Snapshot() DynamicSnapshot {
	if d == nil {
		return DynamicSnapshot{}
	}

	return DynamicSnapshot{
		ID:        d.id,
		CreatedAt: d.createdAt,
		UpdatedAt: d.updatedAt,
		UserID:    d.userID,
		Content:   d.content,
		ImageURL:  d.imageURL,
		LikeCount: d.likeCount,
		IsPinned:  d.isPinned,
		PinnedAt:  d.PinnedAt(),
	}
}

func (d *Dynamic) cloneTime(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}

	cloned := *src

	return &cloned
}

// EnsureDeletableBy 校验删除权限：只有作者本人可删除自己的动态。
func (d *Dynamic) EnsureDeletableBy(userID uint) error {
	if d == nil {
		return ErrDynamicNotFound
	}

	if d.userID != userID {
		return ErrDeleteForbidden
	}

	return nil
}

// EnsurePinnableBy 校验置顶权限：只有作者本人可置顶/取消置顶自己的动态。
func (d *Dynamic) EnsurePinnableBy(userID uint) error {
	if d == nil {
		return ErrDynamicNotFound
	}

	if d.userID != userID {
		return ErrPinForbidden
	}

	return nil
}

// SetPinnedBy 由实体保护作者权限，并保持置顶状态与时间一致。
// 同作者的其它作品置顶由用例在同一事务中清除。失败时保持原状态。
func (d *Dynamic) SetPinnedBy(userID uint, pinned bool, now time.Time) error {
	if err := d.EnsurePinnableBy(userID); err != nil {
		return err
	}

	var pinnedAt *time.Time
	if pinned {
		t := now
		pinnedAt = &t
	}

	d.isPinned = pinned
	d.pinnedAt = pinnedAt

	return nil
}
