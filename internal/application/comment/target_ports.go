package comment

import (
	"context"
)

// RelatedQuery 关联对象的读侧投影端口。
//
// 这些投影直读 users / videos / dynamics / user_focus 表：按 docs/architecture.md 第 4 节，
// 读侧投影允许直读其它上下文的表，实现放在消费方（本上下文）的 infrastructure；
// 写侧（视频计数、通知落库）一律经下方的跨上下文端口。
type RelatedQuery interface {
	// UserProfile 读取 id/username/avatar；不存在时返回 (nil, nil)。
	UserProfile(ctx context.Context, userID uint) (*UserProfile, error)
	// UserProfileByUsername 按用户名读取 id/username/avatar；不存在时返回 (nil, nil)。
	UserProfileByUsername(ctx context.Context, username string) (*UserProfile, error)
	// UsernameByID 读取用户名；不存在或查询失败时返回空串。
	UsernameByID(ctx context.Context, userID uint) string
	// PublishedVideoBrief 读取已发布视频的 id/title/author_username；不存在时返回 (nil, nil)。
	PublishedVideoBrief(ctx context.Context, videoID uint) (*VideoBrief, error)
	// VideoBrief 读取视频 id/title（不限状态）；不存在时返回 (nil, nil)。
	VideoBrief(ctx context.Context, videoID uint) (*VideoBrief, error)
	// VideoAuthorRef 读取视频及其作者用户 ID；视频不存在时返回 (nil, nil)。
	VideoAuthorRef(ctx context.Context, videoID uint) (*VideoAuthorRef, error)
	// DynamicBrief 读取动态 id/user_id；不存在时返回 (nil, nil)。
	DynamicBrief(ctx context.Context, dynamicID uint) (*DynamicBrief, error)
	// AllowedAtUserIDs 过滤出与操作者互相关注（任一方向）的候选 @ 用户；
	// 查询失败时返回 nil。
	AllowedAtUserIDs(ctx context.Context, actorID uint, candidateIDs []uint) []uint
}
