package dynamic

import (
	"context"
)

// UserProfileQuery 用户资料读侧端口（id / username / avatar）。
type UserProfileQuery interface {
	// ProfileByID 读取用户资料摘要；用户不存在时返回 (nil, nil)。
	ProfileByID(ctx context.Context, userID uint) (*UserProfile, error)
	// UsernameByID 读取用户名；用户不存在时返回空串。
	UsernameByID(ctx context.Context, userID uint) (string, error)
}

// VideoOwnershipQuery 视频归属读侧端口。
type VideoOwnershipQuery interface {
	// ExistsPublishedByAuthor 判定视频是否存在、已发布且属于指定作者。
	ExistsPublishedByAuthor(ctx context.Context, videoID uint, authorUsername string) (bool, error)
}
