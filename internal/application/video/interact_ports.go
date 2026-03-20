package video

import (
	"context"
)

// ExpAwarder 经验变动端口。
//
// 绑定目标：identity 上下文的 *identity.ExpService。
type ExpAwarder interface {
	// WatchExp 每日观看视频加经验（每日一次幂等）。
	WatchExp(ctx context.Context, userID uint) error
	// LikeExp 点赞加经验（每日一次幂等）。
	LikeExp(ctx context.Context, userID uint) error
	// CoinExpForVideo 投币加经验（同一用户同一视频仅一次）。
	CoinExpForVideo(ctx context.Context, userID, videoID uint, coins int64) error
}

// CoinLedger 硬币账本端口，绑定到 identity 的 CoinService。
//
// ApplyChange 复用 context 中已有的事务，使三连和投币的硬币变更与视频侧写入处于同一事务。
type CoinLedger interface {
	RevokeFavoriteReward(ctx context.Context, authorID, actorID, videoID uint) error
	// LockAccounts 在调用方事务中锁定账户；由 identity 负责排序与去重。
	LockAccounts(ctx context.Context, userIDs []uint) error
	// ApplyChange 增减用户硬币余额并写入流水。
	ApplyChange(ctx context.Context, userID uint, delta float64, reason string, dedupKey *string) error
	// IsInsufficient 判断错误是否为「硬币余额不足」。
	//
	// 显式暴露判定方法而非共享哨兵错误，避免 video 上下文 import identity 领域包。
	IsInsufficient(err error) bool
}
