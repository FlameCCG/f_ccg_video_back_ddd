package media

import (
	"context"
	"io"

	domain "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// StorageService 提供可复用的存储能力：文件保存、对象删除、分片上传与查询。
//
// 驱动选择、分片校验和路径穿越防护由 infrastructure 的 Resolver / LocalStorage / MinIOStorage 完成。
// SaveXxx 的返回值即公开访问地址：本地驱动返回磁盘绝对路径，MinIO 返回 endpoint+bucket+key 组成的 URL。
type StorageService struct {
	resolver domain.StorageResolver
}

// NewStorageService 构造存储用例服务。
func NewStorageService(resolver domain.StorageResolver) *StorageService {
	return &StorageService{resolver: resolver}
}

// Resolve 按站点配置解析当前生效的存储驱动。
func (s *StorageService) Resolve() (domain.StorageDriver, error) {
	return s.resolver.Resolve()
}

// ResolveByType 按显式存储类型解析驱动；未知类型退回按站点配置解析。
func (s *StorageService) ResolveByType(storageType enum.VideoStorageType) (domain.StorageDriver, error) {
	return s.resolver.ResolveByType(storageType)
}

// CurrentStorageType 返回当前站点启用的存储类型（本地优先）。
func (s *StorageService) CurrentStorageType() enum.VideoStorageType {
	return s.resolver.CheckStorageTyped()
}

// SaveFile 把任意文件写入当前存储，返回可公开访问的路径/URL。
func (s *StorageService) SaveFile(
	ctx context.Context,
	relativePath string,
	size int64,
	reader io.Reader,
	contentType string,
) (string, error) {
	driver, err := s.resolver.Resolve()
	if err != nil {
		return "", err
	}

	return driver.SaveFile(ctx, relativePath, size, reader, contentType)
}

// SaveChunk 写入一个上传分片。
func (s *StorageService) SaveChunk(
	ctx context.Context,
	fileHash, index string,
	size int64,
	reader io.Reader,
) error {
	driver, err := s.resolver.Resolve()
	if err != nil {
		return err
	}

	return driver.SaveChunk(ctx, fileHash, index, size, reader)
}

// ListUploadedChunks 列出指定文件 hash 已上传的分片索引。
func (s *StorageService) ListUploadedChunks(ctx context.Context, fileHash string) ([]string, error) {
	driver, err := s.resolver.Resolve()
	if err != nil {
		return nil, err
	}

	return driver.ListUploadedChunks(ctx, fileHash)
}

// GetCompletedFile 检查最终合并文件是否已存在，并返回其路径/URL。
func (s *StorageService) GetCompletedFile(ctx context.Context, fileHash string) (string, bool, error) {
	driver, err := s.resolver.Resolve()
	if err != nil {
		return "", false, err
	}

	return driver.GetCompletedFile(ctx, fileHash)
}

// MergeChunks 把顺序分片合并成最终文件，返回可访问路径/URL。
func (s *StorageService) MergeChunks(
	ctx context.Context,
	fileHash, fileName string,
	totalChunks int,
) (string, error) {
	driver, err := s.resolver.Resolve()
	if err != nil {
		return "", err
	}

	return driver.MergeChunks(ctx, fileHash, fileName, totalChunks)
}

// DeleteFile 从当前存储删除一个对象；空路径视为 no-op。
func (s *StorageService) DeleteFile(ctx context.Context, storedPath string) error {
	driver, err := s.resolver.Resolve()
	if err != nil {
		return err
	}

	return driver.DeleteStoredFile(ctx, storedPath)
}

// DeleteFileByType 按显式存储类型删除对象（清理历史视频对象时使用）。
func (s *StorageService) DeleteFileByType(
	ctx context.Context,
	storageType enum.VideoStorageType,
	storedPath string,
) error {
	driver, err := s.resolver.ResolveByType(storageType)
	if err != nil {
		return err
	}

	return driver.DeleteStoredFile(ctx, storedPath)
}

// DeleteByPrefix 删除指定前缀下的所有文件，返回删除数量（DASH 目录清理用）。
func (s *StorageService) DeleteByPrefix(ctx context.Context, prefix string) (int64, error) {
	driver, err := s.resolver.Resolve()
	if err != nil {
		return 0, err
	}

	return driver.DeleteByPrefix(ctx, prefix)
}

// DeleteByPrefixWithType 按显式存储类型删除前缀下的所有文件。
func (s *StorageService) DeleteByPrefixWithType(
	ctx context.Context,
	storageType enum.VideoStorageType,
	prefix string,
) (int64, error) {
	driver, err := s.resolver.ResolveByType(storageType)
	if err != nil {
		return 0, err
	}

	return driver.DeleteByPrefix(ctx, prefix)
}

// StatFile 返回已存储对象的字节数；对象不存在或不可访问时返回 error。
func (s *StorageService) StatFile(ctx context.Context, storedPath string) (int64, error) {
	driver, err := s.resolver.Resolve()
	if err != nil {
		return 0, err
	}

	return driver.StatStoredFile(ctx, storedPath)
}
