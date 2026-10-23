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
//
// 回调必须使用传入的 context，以响应失锁或取消；持有锁期间完成验证与引用落库才能缩小删除窗口。
// 多哈希实现须使用稳定顺序获取锁，防止两次多分P投稿以相反顺序互相等待。
type UploadHashGuard interface {
	Run(ctx context.Context, hashes []string, fn func(context.Context) error) error
	TryRun(ctx context.Context, hash string, fn func(context.Context) error) (bool, error)
}

// UploadQuotaService 先确定豁免与日上限，再经存储端口原子占用一个源文件名额。
// 额度单位是用户当天不同的文件哈希，不是 HTTP 请求数、分片数或最终投稿数。
type UploadQuotaService struct {
	store     UploadQuotaStore
	exemption UploadQuotaExemption
	cfg       shared.ConfigReader
}

func NewUploadQuotaService(store UploadQuotaStore, exemption UploadQuotaExemption, cfg shared.ConfigReader) *UploadQuotaService {
	return &UploadQuotaService{store: store, exemption: exemption, cfg: cfg}
}

// Claim 对管理员豁免，对普通用户占用额度；依赖故障返回不可用，不把故障解释为允许上传。
// 同哈希重试是否重复计数由 store 的原子去重契约保证，本方法不在进程内先读后写计数。
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
