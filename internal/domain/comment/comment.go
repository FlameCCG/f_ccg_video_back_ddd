package comment

import (
	"slices"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Comment 评论实体。
//
// 标识、归属、评论对象、正文、置顶和计数都由实体保管。
// HTTP 使用应用层 CommentItem，不直接序列化本类型。
type Comment struct {
	id            uint
	createdAt     time.Time
	updatedAt     time.Time
	content       string
	userID        uint
	videoID       uint
	dynamicID     uint
	commentType   enum.CommentType
	parentID      *uint
	rootID        *uint
	atUserIDs     []uint
	pictures      []string
	likeCount     int64
	replyCount    int64
	isPinned      bool
	pinnedAt      *time.Time
	commenterIP   string
	commenterAddr string
}

// Snapshot 是评论的持久化快照，字段公开，供基础设施重建历史数据。
type Snapshot struct {
	ID            uint
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Content       string
	UserID        uint
	VideoID       uint
	DynamicID     uint
	CommentType   enum.CommentType
	ParentID      *uint
	RootID        *uint
	AtUserIDs     []uint
	Pictures      []string
	LikeCount     int64
	ReplyCount    int64
	IsPinned      bool
	PinnedAt      *time.Time
	CommenterIP   string
	CommenterAddr string
}

// RestoreComment 从快照恢复评论，不重新执行创建校验。
//
// 应用层不要调用。创建走 NewComment。
func RestoreComment(snapshot Snapshot) *Comment {
	return &Comment{
		id:            snapshot.ID,
		createdAt:     snapshot.CreatedAt,
		updatedAt:     snapshot.UpdatedAt,
		content:       snapshot.Content,
		userID:        snapshot.UserID,
		videoID:       snapshot.VideoID,
		dynamicID:     snapshot.DynamicID,
		commentType:   snapshot.CommentType,
		parentID:      snapshot.copyUint(snapshot.ParentID),
		rootID:        snapshot.copyUint(snapshot.RootID),
		atUserIDs:     slices.Clone(snapshot.AtUserIDs),
		pictures:      slices.Clone(snapshot.Pictures),
		likeCount:     snapshot.LikeCount,
		replyCount:    snapshot.ReplyCount,
		isPinned:      snapshot.IsPinned,
		pinnedAt:      snapshot.copyTime(snapshot.PinnedAt),
		commenterIP:   snapshot.CommenterIP,
		commenterAddr: snapshot.CommenterAddr,
	}
}

// Snapshot 返回实体状态的副本，调用方改快照不会写回实体。
func (c *Comment) Snapshot() Snapshot {
	if c == nil {
		return Snapshot{}
	}

	var copy Snapshot

	return Snapshot{
		ID:            c.id,
		CreatedAt:     c.createdAt,
		UpdatedAt:     c.updatedAt,
		Content:       c.content,
		UserID:        c.userID,
		VideoID:       c.videoID,
		DynamicID:     c.dynamicID,
		CommentType:   c.commentType,
		ParentID:      copy.copyUint(c.parentID),
		RootID:        copy.copyUint(c.rootID),
		AtUserIDs:     slices.Clone(c.atUserIDs),
		Pictures:      slices.Clone(c.pictures),
		LikeCount:     c.likeCount,
		ReplyCount:    c.replyCount,
		IsPinned:      c.isPinned,
		PinnedAt:      copy.copyTime(c.pinnedAt),
		CommenterIP:   c.commenterIP,
		CommenterAddr: c.commenterAddr,
	}
}

func (Snapshot) copyUint(id *uint) *uint {
	if id == nil {
		return nil
	}

	value := *id

	return &value
}

func (Snapshot) copyTime(at *time.Time) *time.Time {
	if at == nil {
		return nil
	}

	value := *at

	return &value
}

// ID 返回评论 ID。
func (c *Comment) ID() uint {
	if c == nil {
		return 0
	}

	return c.id
}

// CreatedAt 返回创建时间。
func (c *Comment) CreatedAt() time.Time {
	if c == nil {
		return time.Time{}
	}

	return c.createdAt
}

// UpdatedAt 返回更新时间。
func (c *Comment) UpdatedAt() time.Time {
	if c == nil {
		return time.Time{}
	}

	return c.updatedAt
}

// Content 返回正文。
func (c *Comment) Content() string {
	if c == nil {
		return ""
	}

	return c.content
}

// UserID 返回评论作者 ID。
func (c *Comment) UserID() uint {
	if c == nil {
		return 0
	}

	return c.userID
}

// VideoID 返回关联视频 ID。
func (c *Comment) VideoID() uint {
	if c == nil {
		return 0
	}

	return c.videoID
}

// DynamicID 返回关联动态 ID。
func (c *Comment) DynamicID() uint {
	if c == nil {
		return 0
	}

	return c.dynamicID
}

// CommentType 返回评论类型。
func (c *Comment) CommentType() enum.CommentType {
	if c == nil {
		return ""
	}

	return c.commentType
}

// ParentID 返回父评论 ID 的副本。
func (c *Comment) ParentID() *uint {
	if c == nil {
		return nil
	}

	return (Snapshot{}).copyUint(c.parentID)
}

// RootID 返回根评论 ID 的副本。
func (c *Comment) RootID() *uint {
	if c == nil {
		return nil
	}

	return (Snapshot{}).copyUint(c.rootID)
}

// AtUserIDs 返回 @ 用户 ID 的副本。
func (c *Comment) AtUserIDs() []uint {
	if c == nil {
		return nil
	}

	return slices.Clone(c.atUserIDs)
}

// Pictures 返回图片列表的副本。
func (c *Comment) Pictures() []string {
	if c == nil {
		return nil
	}

	return slices.Clone(c.pictures)
}

// LikeCount 返回点赞数。
func (c *Comment) LikeCount() int64 {
	if c == nil {
		return 0
	}

	return c.likeCount
}

// ReplyCount 返回回复数。
func (c *Comment) ReplyCount() int64 {
	if c == nil {
		return 0
	}

	return c.replyCount
}

// IsPinned 报告评论是否置顶。
func (c *Comment) IsPinned() bool {
	if c == nil {
		return false
	}

	return c.isPinned
}

// PinnedAt 返回置顶时间的副本。
func (c *Comment) PinnedAt() *time.Time {
	if c == nil {
		return nil
	}

	return (Snapshot{}).copyTime(c.pinnedAt)
}

// CommenterIP 返回评论者 IP。
func (c *Comment) CommenterIP() string {
	if c == nil {
		return ""
	}

	return c.commenterIP
}

// CommenterAddr 返回评论者归属地。
func (c *Comment) CommenterAddr() string {
	if c == nil {
		return ""
	}

	return c.commenterAddr
}

// IsRoot 报告该评论是否为一级（根）评论。
func (c *Comment) IsRoot() bool {
	return c != nil && c.rootID == nil
}

// Like 评论点赞记录，字段公开并定义 JSON 响应。
type Like struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
	CommentID uint      `json:"commentId"`
	UserID    uint      `json:"userId"`
}

// Draft 发布评论的输入（值对象，未经规范化）。
type Draft struct {
	UserID        uint
	VideoID       uint
	DynamicID     uint
	Content       string
	ParentID      *uint
	AtUserIDs     []uint
	Pictures      []string
	CommenterIP   string
	CommenterAddr string
}

// ParentBrief 父评论投影：回复时用于推导 rootID 与被回复者信息。
type ParentBrief struct {
	UserID   uint
	RootID   *uint
	Username string
}

// NewComment 校验并规范化草稿，复制可变输入后构造待落库实体。
func NewComment(draft Draft, rootID *uint) (*Comment, error) {
	normalized, commentType, err := draft.Normalize()
	if err != nil {
		return nil, err
	}

	draft = normalized

	return RestoreComment(Snapshot{
		Content:       draft.Content,
		UserID:        draft.UserID,
		VideoID:       draft.VideoID,
		DynamicID:     draft.DynamicID,
		CommentType:   commentType,
		ParentID:      draft.ParentID,
		RootID:        rootID,
		AtUserIDs:     draft.AtUserIDs,
		Pictures:      draft.Pictures,
		CommenterIP:   draft.CommenterIP,
		CommenterAddr: draft.CommenterAddr,
	}), nil
}

// PinBy 校验作品作者资格后，同时维护置顶标记和时间。
// 同一作品只能有一条置顶评论，由调用方在同一事务内解除旧置顶并保存本实体。
func (c *Comment) PinBy(isWorkAuthor, pinned bool, now time.Time) error {
	if err := c.EnsurePinnable(); err != nil {
		return err
	}
	if !isWorkAuthor {
		return ErrPinForbidden
	}

	c.isPinned = pinned
	c.pinnedAt = nil

	if pinned {
		pinnedAt := now
		c.pinnedAt = &pinnedAt
	}

	return nil
}
