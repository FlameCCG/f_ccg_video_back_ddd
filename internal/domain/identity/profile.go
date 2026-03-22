package identity

import (
	"time"
)

// ProfileInput 是资料更新命令，字段公开。空串与 nil 表示本次不更新。
type ProfileInput struct {
	Username    string
	Avatar      string
	Description string
	Gender      *int
	Birthday    string
}

// EnsureUsernameAvailable 根据排除自身后的占用事实校验用户名。
func (u *User) EnsureUsernameAvailable(occupied int64) error {
	if u == nil {
		return ErrUserNotFound
	}
	if occupied > 0 {
		return ErrUsernameExists
	}
	return nil
}

// ProfileChanges 用户资料的部分更新集合；nil 字段表示本次不更新。
type ProfileChanges struct {
	Username    *string
	Avatar      *string
	Description *string
	Gender      *int
	Birthday    *time.Time
}

// IsEmpty 报告是否没有任何字段需要更新。
func (c ProfileChanges) IsEmpty() bool {
	return c.Username == nil &&
		c.Avatar == nil &&
		c.Description == nil &&
		c.Gender == nil &&
		c.Birthday == nil
}
