// Package social 是社交关系上下文的领域层：关注关系聚合、
// 关注/粉丝列表的可见性规则与仓储接口（纯 Go，零框架依赖）。
package social

import "time"

// Focus 关注关系聚合根：UserID 关注了 FocusUserID。
type Focus struct {
	ID          uint
	CreatedAt   time.Time
	UpdatedAt   time.Time
	UserID      uint
	FocusUserID uint
}

// NewFocus 校验并构造一条关注关系。
//
// 规则与原 api/social_api/focus_user.go 一致：不允许关注自己。
func NewFocus(userID, focusUserID uint) (*Focus, error) {
	if userID == focusUserID {
		return nil, ErrCannotFocusSelf
	}

	return &Focus{UserID: userID, FocusUserID: focusUserID}, nil
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
