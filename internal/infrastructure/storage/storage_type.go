package storage

import (
	"errors"

	media "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/config/site"
	"f_ccg_video_back_ddd/pkg/ctxutil"
)

// Resolver 依据站点配置解析存储驱动（原 storage_service.ResolveDriver、
// mq_service.resolveDriverByType 与 pkg.CheckStorageTyped 的合并实现）。
//
// 站点存储配置支持运行期热更新，因此每次解析都从 *config.Provider 重新读取，
// 与原实现「每个调用点重新读 global.Config()」的语义一致。
type Resolver struct {
	cfgProvider *config.Provider
	clients     *MinIOClientPool
	detached    *ctxutil.DetachedTaskGroup
}

// 编译期断言：满足领域侧存储解析端口。
var _ media.StorageResolver = (*Resolver)(nil)

// NewResolver 构造存储驱动解析器。
func NewResolver(
	cfgProvider *config.Provider,
	clients *MinIOClientPool,
	detached *ctxutil.DetachedTaskGroup,
) *Resolver {
	return &Resolver{cfgProvider: cfgProvider, clients: clients, detached: detached}
}

// storageConfig 读取当前站点存储配置。
func (r *Resolver) storageConfig() (*site.Storage, error) {
	if r == nil || r.cfgProvider == nil {
		return nil, errors.New("未找到存储配置")
	}

	cfg := r.cfgProvider.Get()
	if cfg == nil || cfg.Site == nil || cfg.Site.Storage == nil {
		return nil, errors.New("未找到存储配置")
	}

	return cfg.Site.Storage, nil
}

// Resolve 按站点配置选择本地或 MinIO 存储驱动。
func (r *Resolver) Resolve() (media.StorageDriver, error) {
	cfg, err := r.storageConfig()
	if err != nil {
		return nil, err
	}

	if cfg.Local != nil && cfg.Local.Enable {
		return NewLocalStorage(cfg), nil
	}

	if cfg.MinIO != nil && cfg.MinIO.Enable {
		return NewMinIOStorage(cfg, r.clients, r.detached)
	}

	return nil, errors.New("未配置可用的存储驱动")
}

// ResolveByType 按显式存储类型选择驱动；未知类型退回按站点配置解析。
func (r *Resolver) ResolveByType(storageType enum.VideoStorageType) (media.StorageDriver, error) {
	cfg, err := r.storageConfig()
	if err != nil {
		// 与原 resolveDriverByType 的错误文案保持一致。
		return nil, errors.New("storage config missing")
	}

	switch storageType {
	case enum.StorageTypeLocal:
		if cfg.Local == nil || !cfg.Local.Enable {
			return nil, errors.New("local storage disabled")
		}

		return NewLocalStorage(cfg), nil
	case enum.StorageTypeMinio:
		if cfg.MinIO == nil || !cfg.MinIO.Enable {
			return nil, errors.New("minio storage disabled")
		}

		return NewMinIOStorage(cfg, r.clients, r.detached)
	default:
		return r.Resolve()
	}
}

// CheckStorageTyped 返回当前站点启用的存储类型（本地优先，两者都未开启时回落本地）。
func (r *Resolver) CheckStorageTyped() enum.VideoStorageType {
	cfg, err := r.storageConfig()
	if err != nil {
		return enum.StorageTypeLocal
	}

	if cfg.Local != nil && cfg.Local.Enable {
		return enum.StorageTypeLocal
	}

	if cfg.MinIO != nil && cfg.MinIO.Enable {
		return enum.StorageTypeMinio
	}

	return enum.StorageTypeLocal
}

// ValidateUploadID 校验上传标识（供其它上下文经端口复用，实现与包级函数一致）。
func (r *Resolver) ValidateUploadID(uploadID string) error {
	return ValidateUploadID(uploadID)
}

// ValidateChunkIndex 校验分片索引（供其它上下文经端口复用）。
func (r *Resolver) ValidateChunkIndex(index string) error {
	return ValidateChunkIndex(index)
}
