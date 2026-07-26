package platform

import (
	"context"

	"go.uber.org/zap"
)

// videoStatsSyncBatchSize 单轮脏集合扫描的批大小（与原实现的 200 一致）。
const videoStatsSyncBatchSize = 200

// VideoStatSyncService 视频统计缓存回写用例（原 service/cron_service/sync_video_stat.go）。
type VideoStatSyncService struct {
	cache      VideoStatsCache
	projection VideoStatsProjection
}

// NewVideoStatSyncService 构造视频统计回写用例服务。
func NewVideoStatSyncService(cache VideoStatsCache, projection VideoStatsProjection) *VideoStatSyncService {
	return &VideoStatSyncService{cache: cache, projection: projection}
}

// Run 将 Redis 中的脏视频统计同步到数据库，按游标分批直到扫描一轮结束。
//
// 扫描失败直接结束本轮；快照加载失败只记录警告并继续处理已取到的部分
// （与原实现一致：loadErr 不中断本批同步）。
func (s *VideoStatSyncService) Run(ctx context.Context) {
	if !s.cache.Available() {
		zap.L().Warn("skip SyncVideoStat: redis not configured")

		return
	}

	// Cron 任务是调度 root 场景，没有请求上下文。
	if ctx == nil {
		ctx = context.Background()
	}

	var cursor uint64

	for {
		ids, nextCursor, err := s.cache.ScanDirty(ctx, cursor, videoStatsSyncBatchSize)
		if err != nil {
			zap.L().Warn("scan dirty video stats failed", zap.Error(err))

			return
		}

		if len(ids) > 0 {
			snapshots, loadErr := s.cache.LoadSnapshots(ctx, ids)
			if loadErr != nil {
				zap.L().Warn("load dirty video stats failed", zap.Error(loadErr))
			}

			s.syncSnapshots(ctx, snapshots)
		}

		cursor = nextCursor
		if cursor == 0 {
			return
		}
	}
}

// syncSnapshots 逐个回写快照：落库成功后才 CAS 摘除脏标记。
func (s *VideoStatSyncService) syncSnapshots(ctx context.Context, snapshots []VideoStatsSnapshot) {
	for _, snapshot := range snapshots {
		if snapshot.VideoID == 0 || snapshot.Stats == nil {
			continue
		}

		if err := s.projection.PersistStats(ctx, snapshot.VideoID, *snapshot.Stats); err != nil {
			zap.L().Error("persist video stats failed", zap.Uint("videoId", snapshot.VideoID), zap.Error(err))

			continue
		}

		if err := s.cache.AckVersion(ctx, snapshot.MemberID, snapshot.Version); err != nil {
			zap.L().Warn("ack dirty video stats version failed", zap.Uint("videoId", snapshot.VideoID), zap.Error(err))
		}
	}
}
