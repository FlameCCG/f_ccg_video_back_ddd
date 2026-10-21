package video

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"f_ccg_video_back_ddd/internal/application/shared"
)

const defaultDailyUploadLimit = 10

var (
	ErrDailyUploadLimit       = errors.New("今日上传源视频文件数量已达上限，请明天再试")
	ErrUploadQuotaUnavailable = errors.New("上传额度服务暂不可用，请稍后重试")
)

// UploadQuotaStore 在 Redis 中按北京时间自然日原子记录用户已占用的文件哈希。
type UploadQuotaStore interface {
	Claim(ctx context.Context, userID uint, fileHash string, limit int) (bool, error)
}

// UploadQuotaExemption 是本上下文需要的额度豁免判定，不暴露身份上下文的角色模型。
type UploadQuotaExemption interface {
	IsUploadQuotaExempt(ctx context.Context, userID uint) (bool, error)
}

// UploadHashGuard 在上传、投稿与清理间按哈希协调。TryRun 不等待在途操作。
type UploadHashGuard interface {
	Run(ctx context.Context, hashes []string, fn func(context.Context) error) error
	TryRun(ctx context.Context, hash string, fn func(context.Context) error) (bool, error)
}

type UploadQuotaService struct {
	store     UploadQuotaStore
	exemption UploadQuotaExemption
	cfg       shared.ConfigReader
}

func NewUploadQuotaService(store UploadQuotaStore, exemption UploadQuotaExemption, cfg shared.ConfigReader) *UploadQuotaService {
	return &UploadQuotaService{store: store, exemption: exemption, cfg: cfg}
}

func (s *UploadQuotaService) Claim(ctx context.Context, userID uint, fileHash string) error {
	fileHash = strings.TrimSpace(fileHash)
	if s == nil || s.store == nil || s.exemption == nil || userID == 0 {
		return ErrUploadQuotaUnavailable
	}
	admin, err := s.exemption.IsUploadQuotaExempt(ctx, userID)
	if err != nil {
		return ErrUploadQuotaUnavailable
	}
	if admin {
		return nil
	}
	limit := defaultDailyUploadLimit
	if s.cfg != nil {
		if cfg := s.cfg.Get(); cfg != nil && cfg.Site != nil && cfg.Site.Storage != nil && cfg.Site.Storage.DailyUploadLimit > 0 {
			limit = cfg.Site.Storage.DailyUploadLimit
		}
	}
	allowed, err := s.store.Claim(ctx, userID, fileHash, limit)
	if err != nil {
		return ErrUploadQuotaUnavailable
	}
	if !allowed {
		return fmt.Errorf("%w（每日最多 %d 个不同文件）", ErrDailyUploadLimit, limit)
	}
	return nil
}

func uniqueUploadHashes(hashes []string) []string {
	seen := make(map[string]struct{}, len(hashes))
	unique := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		hash = strings.TrimSpace(hash)
		if hash == "" {
			continue
		}
		if _, exists := seen[hash]; !exists {
			seen[hash] = struct{}{}
			unique = append(unique, hash)
		}
	}
	sort.Strings(unique)
	return unique
}
