package dynamic

import (
	"time"
)

// UserProfile 用户资料摘要（id / username / avatar）。
type UserProfile struct {
	ID       uint
	Username string
	Avatar   string
}

// VideoAuthorRef 已发布视频的作者引用。
type VideoAuthorRef struct {
	VideoID        uint
	VideoAt        time.Time
	AuthorID       uint
	AuthorUsername string
	VideoStatus    int
}
