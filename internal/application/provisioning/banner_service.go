package provisioning

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"

	"go.uber.org/zap"
)

const (
	// bannerImagesDir 初始化横幅图片所在目录（与原实现一致，相对工作目录）。
	bannerImagesDir = "init_resource/images"
	// bannerStoragePrefix 上传到存储后的对象路径前缀（外部契约，不可更改）。
	bannerStoragePrefix = "images/banners/"
	// bannerFallbackContentType 扩展名无法识别时的兜底 MIME。
	bannerFallbackContentType = "image/png"
	// bannerUserKeyword 文件名包含该关键字视为用户配置横幅。
	bannerUserKeyword = "用户配置横幅"
	// bannerHomeKeyword 文件名包含该关键字视为首页轮播。
	bannerHomeKeyword = "首页轮播"
)

// ErrBannerImagesDirUnreadable 图片目录不可读。
var ErrBannerImagesDirUnreadable = errors.New("读取横幅图片目录失败")

// BannerService Banner 初始化用例（原 flags.FlagBannerInit）。
type BannerService struct {
	storage StorageDriver
	repo    BannerRepository
	logger  *zap.Logger
}

// NewBannerService 构造 Banner 初始化用例。
func NewBannerService(storage StorageDriver, repo BannerRepository, logger *zap.Logger) *BannerService {
	return &BannerService{storage: storage, repo: repo, logger: logger}
}

// Provision 扫描 init_resource/images，把匹配的图片上传到存储并幂等写入 banner 表。
//
// 返回成功处理的条数。与原实现一致：单个文件失败只记日志并继续，不中断整体流程。
func (s *BannerService) Provision(ctx context.Context) (int, error) {
	files, err := os.ReadDir(bannerImagesDir)
	if err != nil {
		return 0, fmt.Errorf("%w: %s: %w", ErrBannerImagesDirUnreadable, bannerImagesDir, err)
	}

	if len(files) == 0 {
		s.logger.Warn("No images found in images directory", zap.String("dir", bannerImagesDir))

		return 0, nil
	}

	// Make sure banner table is migrated/exists
	if err := s.repo.MigrateBannerTable(ctx); err != nil {
		return 0, fmt.Errorf("Failed to migrate banner table: %w", err) //nolint:staticcheck // 保留原日志文案
	}

	var successCount int

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()

		bannerType, ok := classifyBannerFile(filename)
		if !ok {
			// Skip files that don't match the criteria
			s.logger.Debug("Skipping unrelated file in images folder", zap.String("filename", filename))

			continue
		}

		publicURL, err := s.upload(ctx, filename)
		if err != nil {
			s.logger.Error("Failed to upload banner to storage",
				zap.String("filename", filename), zap.Error(err))

			continue
		}

		created, err := s.repo.UpsertBannerByCover(ctx, publicURL, bannerType)
		if err != nil {
			s.logger.Error("Failed to persist banner record",
				zap.String("filename", filename), zap.Error(err))

			continue
		}

		if created {
			s.logger.Info("Created new banner record",
				zap.String("filename", filename),
				zap.String("url", publicURL),
				zap.Int("type", int(bannerType)))
		} else {
			s.logger.Info("Updated existing banner record",
				zap.String("filename", filename),
				zap.String("url", publicURL))
		}

		successCount++
	}

	return successCount, nil
}

// upload 打开图片并写入存储，返回公开访问 URL。
func (s *BannerService) upload(ctx context.Context, filename string) (string, error) {
	filePath := filepath.Join(bannerImagesDir, filename)

	file, err := os.Open(filePath) //nolint:gosec // 路径来自受控的 init_resource 目录
	if err != nil {
		return "", fmt.Errorf("Failed to open file %s: %w", filePath, err) //nolint:staticcheck // 保留原日志文案
	}

	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("Failed to stat file %s: %w", filePath, err) //nolint:staticcheck // 保留原日志文案
	}

	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = bannerFallbackContentType // fallback
	}

	// Relative path inside storage: images/banners/filename
	return s.storage.SaveFile(ctx, bannerStoragePrefix+filename, info.Size(), file, contentType)
}

// classifyBannerFile 依据文件名推断横幅类型（规则与原实现一致）。
func classifyBannerFile(filename string) (enum.BannerType, bool) {
	switch {
	case strings.Contains(filename, bannerUserKeyword):
		return enum.BannerTypeUser, true // 3
	case strings.Contains(filename, bannerHomeKeyword):
		return enum.BannerTypeHome, true // 1
	default:
		return 0, false
	}
}
