package video

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/internal/application/shared"
	domain "f_ccg_video_back_ddd/internal/domain/video"

	"go.uber.org/zap"
)

type UploadGarbageKind string
type UploadGarbageLocation string

const (
	UploadGarbageChunks UploadGarbageKind     = "chunks"
	UploadGarbageSource UploadGarbageKind     = "source"
	UploadGarbageLocal  UploadGarbageLocation = "local"
	UploadGarbageMinIO  UploadGarbageLocation = "minio"
)

type UploadGarbage struct {
	Kind         UploadGarbageKind
	Location     UploadGarbageLocation
	FileHash     string
	Key          string
	Root         string
	SourcePath   string
	LastModified time.Time
}

type UploadGarbageScanner interface {
	List(ctx context.Context, chunkBefore, sourceBefore time.Time) ([]UploadGarbage, error)
	DeleteIfExpired(ctx context.Context, item UploadGarbage, before time.Time) (bool, error)
}

// UploadCleanupService 扫描上线前遗留对象，并在删除源文件前只读查询现有视频资源引用。
type UploadCleanupService struct {
	scanner UploadGarbageScanner
	videos  domain.VideoRepository
	guard   UploadHashGuard
	cfg     shared.ConfigReader
	logger  *zap.Logger
}

func NewUploadCleanupService(scanner UploadGarbageScanner, videos domain.VideoRepository, guard UploadHashGuard, cfg shared.ConfigReader, logger *zap.Logger) *UploadCleanupService {
	return &UploadCleanupService{scanner: scanner, videos: videos, guard: guard, cfg: cfg, logger: logger}
}

func (s *UploadCleanupService) Run(ctx context.Context) {
	if s == nil || s.scanner == nil || s.guard == nil {
		return
	}
	chunkHours, sourceHours := 2, 1
	if s.cfg != nil {
		if cfg := s.cfg.Get(); cfg != nil && cfg.Site != nil && cfg.Site.Storage != nil {
			if n := cfg.Site.Storage.UnmergedChunkRetentionHours; n > 0 {
				chunkHours = n
			}
			if n := cfg.Site.Storage.UnpublishedSourceRetentionHours; n > 0 {
				sourceHours = n
			}
		}
	}
	now := time.Now()
	chunkBefore := now.Add(-time.Duration(chunkHours) * time.Hour)
	sourceBefore := now.Add(-time.Duration(sourceHours) * time.Hour)
	items, scanErr := s.scanner.List(ctx, chunkBefore, sourceBefore)
	if scanErr != nil {
		s.log().Warn("scan expired uploads failed", zap.Error(scanErr))
	}
	for _, item := range items {
		if ctx.Err() != nil {
			return
		}
		before := chunkBefore
		if item.Kind == UploadGarbageSource {
			before = sourceBefore
		}
		_, err := s.guard.TryRun(ctx, item.FileHash, func(guardCtx context.Context) error {
			if item.Kind == UploadGarbageSource {
				referenced, queryErr := s.videos.StoragePathStillReferenced(guardCtx, item.SourcePath)
				if queryErr != nil {
					return queryErr
				}
				if referenced {
					return nil
				}
			}
			deleted, deleteErr := s.scanner.DeleteIfExpired(guardCtx, item, before)
			if deleted {
				s.log().Info("expired upload removed", zap.String("hash", item.FileHash), zap.String("key", item.Key))
			}
			return deleteErr
		})
		if err != nil {
			s.log().Warn("skip expired upload deletion", zap.String("hash", item.FileHash), zap.Error(err))
		}
	}
}

func (s *UploadCleanupService) log() *zap.Logger {
	if s.logger != nil {
		return s.logger
	}
	return zap.NewNop()
}
