package social

import "time"

// Focus 关注关系聚合根：用户关注了另一位用户。
type Focus struct {
	id          uint
	createdAt   time.Time
	updatedAt   time.Time
	userID      uint
	focusUserID uint
}

// FocusSnapshot 关注关系的持久化快照。
//
// 字段公开，供基础设施重建历史数据。应用层不使用该结构恢复实体。
type FocusSnapshot struct {
	ID          uint
	CreatedAt   time.Time
	UpdatedAt   time.Time
	UserID      uint
	FocusUserID uint
}

// NewFocus 校验并构造一条关注关系。
//
// 规则：不允许关注自己。
// 失败时不返回实体。用户 ID 与被关注用户 ID 同时写入。
func NewFocus(userID, focusUserID uint) (*Focus, error) {
	if userID == focusUserID {
		return nil, ErrCannotFocusSelf
	}

	return &Focus{userID: userID, focusUserID: focusUserID}, nil
}

// RestoreFocus 从持久化快照恢复关注关系，不重新执行创建校验。
func RestoreFocus(snapshot FocusSnapshot) *Focus {
	return &Focus{
		id:          snapshot.ID,
		createdAt:   snapshot.CreatedAt,
		updatedAt:   snapshot.UpdatedAt,
		userID:      snapshot.UserID,
		focusUserID: snapshot.FocusUserID,
	}
}

// ID 返回关注关系主键。
func (f *Focus) ID() uint {
	if f == nil {
		return 0
	}

	return f.id
}

// CreatedAt 返回创建时间的副本。
func (f *Focus) CreatedAt() time.Time {
	if f == nil {
		return time.Time{}
	}

	return f.createdAt
}

// UpdatedAt 返回更新时间的副本。
func (f *Focus) UpdatedAt() time.Time {
	if f == nil {
		return time.Time{}
	}

	return f.updatedAt
}

// UserID 返回发起关注的用户 ID。
func (f *Focus) UserID() uint {
	if f == nil {
		return 0
	}

	return f.userID
}

// FocusUserID 返回被关注用户 ID。
func (f *Focus) FocusUserID() uint {
	if f == nil {
		return 0
	}

	return f.focusUserID
}

// RelationCounts 用户的关注/粉丝/动态计数。
type RelationCounts struct {
	FocusCount   int64
	FansCount    int64
	DynamicCount int64
}

// Relation 两个用户之间的关注关系视角（以「我」为主体）。
type Relation struct {
	// IsFocus 我是否关注了他。
	IsFocus bool
	// IsFans 他是否关注了我。
	IsFans bool
}

// IsMutual 是否互相关注。
func (r Relation) IsMutual() bool {
	return r.IsFocus && r.IsFans
}

// Snapshot 返回关注关系快照的副本。修改返回值不会影响实体。
func (f *Focus) Snapshot() FocusSnapshot {
	if f == nil {
		return FocusSnapshot{}
	}

	return FocusSnapshot{
		ID:          f.id,
		CreatedAt:   f.createdAt,
		UpdatedAt:   f.updatedAt,
		UserID:      f.userID,
		FocusUserID: f.focusUserID,
	}
}
