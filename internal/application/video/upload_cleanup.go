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

// UploadGarbage 是扫描得到的清理候选快照，不代表此刻可以安全删除。
// 从扫描到删除之间可能有新上传或投稿，必须持同哈希锁重新检查时效与资源引用。
type UploadGarbage struct {
	Kind         UploadGarbageKind
	Location     UploadGarbageLocation
	FileHash     string
	Key          string
	Root         string
	SourcePath   string
	LastModified time.Time
}

// UploadGarbageScanner 负责技术层扫描与删除时复核过期条件；业务引用检查由用例负责。
type UploadGarbageScanner interface {
	List(ctx context.Context, chunkBefore, sourceBefore time.Time) ([]UploadGarbage, error)
	DeleteIfExpired(ctx context.Context, item UploadGarbage, before time.Time) (bool, error)
}

// UploadCleanupService 清理过期未合并分片及未被稿件引用的源文件。
// 扫描、哈希锁、数据库引用检查和删除时效复核共同构成流程，任一步失败都不应推断文件可删。
type UploadCleanupService struct {
	scanner UploadGarbageScanner
	videos  domain.Repository
	guard   UploadHashGuard
	cfg     shared.ConfigReader
	logger  *zap.Logger
}

func NewUploadCleanupService(scanner UploadGarbageScanner, videos domain.Repository, guard UploadHashGuard, cfg shared.ConfigReader, logger *zap.Logger) *UploadCleanupService {
	return &UploadCleanupService{scanner: scanner, videos: videos, guard: guard, cfg: cfg, logger: logger}
}

// Run 用配置保留期扫描候选，并以非等待锁跳过正在上传或投稿的哈希。
// 源文件删除前检查现有视频资源引用；扫描和单项错误记日志，不阻止本轮处理其他候选。
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
