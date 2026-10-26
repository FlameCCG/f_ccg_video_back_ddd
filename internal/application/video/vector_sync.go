package video

import (
	"context"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/pkg/ctxutil"
)

// VectorSyncService 视频向量索引同步用例服务（原 service/video_service/vector_sync.go）。
type VectorSyncService struct {
	indexer   VectorIndexer
	rebuilder VectorIndexRebuilder
	tasks     *ctxutil.DetachedTaskGroup
}

// NewVectorSyncService 构造向量索引同步用例服务。
func NewVectorSyncService(
	indexer VectorIndexer,
	rebuilder VectorIndexRebuilder,
	tasks *ctxutil.DetachedTaskGroup,
) *VectorSyncService {
	return &VectorSyncService{indexer: indexer, rebuilder: rebuilder, tasks: tasks}
}

// SyncAsync 异步同步单个视频的向量索引：已发布则写入，否则删除。
func (s *VectorSyncService) SyncAsync(parent context.Context, videoID uint, status enum.VideoStatus) {
	if videoID == 0 {
		return
	}

	s.tasks.Go(parent, ctxutil.AsyncLongTimeout, func(ctx context.Context) {
		if status == enum.VideoStatusPublished {
			s.indexer.IndexVideoVectorBestEffort(ctx, videoID)

			return
		}

		s.indexer.DeleteVideoVectorBestEffort(ctx, videoID)
	})
}

// SyncBatchAsync 异步批量同步视频向量索引。
func (s *VectorSyncService) SyncBatchAsync(parent context.Context, videoIDs []uint, status enum.VideoStatus) {
	if len(videoIDs) == 0 {
		return
	}

	s.tasks.Go(parent, ctxutil.AsyncLongTimeout, func(ctx context.Context) {
		for _, videoID := range videoIDs {
			if videoID == 0 {
				continue
			}

			if status == enum.VideoStatusPublished {
				s.indexer.IndexVideoVectorBestEffort(ctx, videoID)

				continue
			}

			s.indexer.DeleteVideoVectorBestEffort(ctx, videoID)
		}
	})
}

// StartRebuild 触发向量索引全量重建；已有任务在跑时返回 ErrVectorRebuildInProgress。
func (s *VectorSyncService) StartRebuild(opts VectorRebuildOptions) (any, error) {
	started, err := s.rebuilder.StartRebuild(opts)
	if err != nil {
		return nil, err
	}

	if !started {
		return nil, ErrVectorRebuildInProgress
	}

	return s.rebuilder.RebuildStatus(), nil
}

// RebuildStatus 查询向量索引重建进度快照。
func (s *VectorSyncService) RebuildStatus() any {
	return s.rebuilder.RebuildStatus()
}
