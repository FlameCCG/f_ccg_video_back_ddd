package video

import (
	"context"
)

// TagRepository 按业务能力声明的持久化契约。
type TagRepository interface {
	// ReplaceTags 覆盖视频的标签关联（tags 为空表示清空关联）。
	ReplaceTags(ctx context.Context, videoID uint, tags []Tag) error
	// TagsByVideoID 读取视频当前关联的标签。
	TagsByVideoID(ctx context.Context, videoID uint) ([]Tag, error)
	// FindTagsByIDs 按 ID 批量读取标签（无匹配时返回非 nil 空切片，与 GORM Find 语义一致）。
	FindTagsByIDs(ctx context.Context, tagIDs []uint) ([]Tag, error)
	// CreateOrGetTag 创建标签（若已存在同名标签则返回现有）。
	CreateOrGetTag(ctx context.Context, name string) (*Tag, error)
}
