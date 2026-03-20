package entity

import (
	"time"
)

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
