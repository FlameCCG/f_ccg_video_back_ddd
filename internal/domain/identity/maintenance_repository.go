package identity

import (
	"context"
	"time"
)

type BanMaintenanceRepository interface {
	// ListExpiredBans 查询每个用户最新一条封禁记录中已到期、且用户仍处于封禁态的条目。
	ListExpiredBans(ctx context.Context, now time.Time) ([]ExpiredBan, error)
	// LatestBansForUpdate 在调用方事务内按用户 ID 加锁，取得每个候选的最新用户和封禁记录。
	LatestBansForUpdate(ctx context.Context, userIDs []uint) ([]BanReleaseState, error)
	// ActivateBannedUsers 将仍处于封禁态的指定用户批量置为正常状态。
	ActivateBannedUsers(ctx context.Context, userIDs []uint) error
	// MarkBanRecordsUnbanned 批量标记原封禁记录已解封。
	MarkBanRecordsUnbanned(ctx context.Context, recordIDs []uint) error
}
