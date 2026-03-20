package video

import (
	"context"
)

// DynamicReadMarker 动态已读标记端口。
//
// 绑定目标：dynamic 上下文。
type DynamicReadMarker interface {
	MaybeMarkReadByVideo(ctx context.Context, userID, videoID uint) error
}

// ContentAuthorizer validates current account status for every content mutation,
// including messages received on an already established WebSocket connection.
type ContentAuthorizer interface {
	EnsureCanPublish(context.Context, uint) error
}
