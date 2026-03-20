package comment

import (
	"context"
)

// LikeCache 评论点赞缓存端口（Redis 实现在 infrastructure/cache/comment）。
//
// Redis key 前缀、TTL 与 Lua 原子切换脚本属于外部契约。
type LikeCache interface {
	// Available 报告底层 Redis 是否可用。
	Available() bool
	// Complete 判断某评论的 count 与 users set 是否成套（半残视为不完整）。
	Complete(ctx context.Context, commentID uint) (bool, error)
	// Write 以给定点赞用户全量重写 set + count（同一 TTL）。
	Write(ctx context.Context, commentID uint, userIDs []uint) error
	// Clear 删除 count/set 两个 key（不写入 count=0）。
	Clear(ctx context.Context, commentID uint) error
	// Drop 尽力删除 count/set 两个 key，失败仅记录日志（用于写后失效）。
	Drop(ctx context.Context, commentID uint)
	// Toggle 用 Lua 原子切换点赞状态，返回最新点赞数与是否已点赞，并写入回写队列。
	Toggle(ctx context.Context, commentID, userID uint) (int64, bool, error)
	// Counts 批量读取点赞数；整体失败返回 (nil, 全部 ID)，部分缺失返回命中项与缺失 ID。
	Counts(ctx context.Context, commentIDs []uint) (map[uint]int64, []uint)
	// LikedFlags 批量读取点赞状态；整体失败返回 (nil, 全部 ID)，缓存不完整项记为缺失。
	LikedFlags(ctx context.Context, commentIDs []uint, userID uint) (map[uint]bool, []uint)
	// ReplyCounts 批量读取回复数缓存；缺失项由调用方用 DB 字段兜底。
	ReplyCounts(ctx context.Context, commentIDs []uint) map[uint]int64
}
