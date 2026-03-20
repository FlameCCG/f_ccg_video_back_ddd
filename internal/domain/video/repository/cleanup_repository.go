package repository

import (
	"context"
)

// CleanupRepository 按业务能力声明的持久化契约。
type CleanupRepository interface {
	// HardDelete 物理删除视频及其全部关联数据，返回各表删除计数与待清理的存储目标。
	HardDelete(ctx context.Context, videoIDs []uint) (*HardDeleteResult, []StorageDeleteTarget, error)
	// StoragePathStillReferenced 判断对象路径是否仍被其它视频/资源引用，兼容历史完整 URL/bucket 前缀。
	StoragePathStillReferenced(ctx context.Context, storedPath string) (bool, error)
}
