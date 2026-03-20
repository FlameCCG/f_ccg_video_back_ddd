package video

import (
	"context"
	"time"
)

// DanmuQuery 弹幕读模型查询端口。
type DanmuQuery interface {
	// History 按播放时间区间读取历史弹幕。
	History(ctx context.Context, videoID, partID uint, start, end, limit int) ([]DanmuDTO, error)
	// RoomList 读取房间弹幕（分段模式按播放时间升序分页；历史模式按创建时间倒序取最近 N 条）。
	RoomList(ctx context.Context, params DanmuRoomQueryParams) ([]DanmuRoomItem, int64, error)
	// AdminList 管理端弹幕列表。
	AdminList(ctx context.Context, params AdminDanmuListParams) ([]AdminDanmuItem, int64, error)
	// CreatorList 创作中心弹幕列表（按作者用户名过滤）。
	CreatorList(ctx context.Context, authorUsername string, page, pageSize, sort int, keyword string) ([]CreatorDanmuItem, int64, error)
	// MonthInstants 返回 [start, end) 内每条弹幕的 created_at。时间点带数据库时区。
	MonthInstants(ctx context.Context, videoID, partID uint, start, end time.Time) ([]time.Time, error)
}

// DanmuRecentCache 最近弹幕列表的 Redis 缓存端口。
type DanmuRecentCache interface {
	// Available 报告 Redis 是否可用。
	Available() bool
	// Push 追加一条弹幕并裁剪列表长度、续期 TTL。
	Push(ctx context.Context, dto *DanmuDTO)
	// Invalidate 删除某房间的最近弹幕缓存。
	Invalidate(ctx context.Context, videoID, partID uint)
}

// DanmuBroadcaster 弹幕广播端口。
//
// 实现是 internal/interfaces/ws/danmu 的 *Hub：它既向本实例的 WebSocket 房间推送，
// 也经 Redis Pub/Sub 广播到其它实例。bootstrap 在构造 Hub 后调用
// DanmuService.SetBroadcaster 回填，以打断「Hub → Service → Hub」的构造环。
type DanmuBroadcaster interface {
	BroadcastDanmu(ctx context.Context, dto *DanmuDTO)
}
