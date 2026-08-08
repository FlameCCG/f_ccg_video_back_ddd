package video

import "strings"

// DefaultFolderName 默认收藏夹名称（与原 folder_service.go 一致）。
const DefaultFolderName = "默认收藏夹"

// FavoriteFolder 收藏夹实体。
type FavoriteFolder struct {
	ID        uint
	UserID    uint
	Name      string
	IsDefault bool
}

// CanDelete 判断收藏夹是否允许删除（默认收藏夹不可删除）。
func (f *FavoriteFolder) CanDelete() error {
	if f == nil {
		return ErrRecordNotFound
	}

	if f.IsDefault {
		return ErrDefaultFolderUndeletable
	}

	return nil
}

// Rename 重命名收藏夹：校验新名称后更新实体。
func (f *FavoriteFolder) Rename(name string) error {
	if f == nil {
		return ErrRecordNotFound
	}

	if err := ValidateFolderName(name); err != nil {
		return err
	}

	f.Name = name

	return nil
}

// ValidateFolderName 校验收藏夹名称。
//
// 原实现只判断空串（未做 TrimSpace），此处保持一致以免改变可观测行为。
func ValidateFolderName(name string) error {
	if name == "" {
		return ErrFolderNameEmpty
	}

	return nil
}

// NormalizeFolderVideoSort 归一化收藏夹内视频排序值，非法值回落最近收藏。
func NormalizeFolderVideoSort(sort int) int {
	switch sort {
	case FolderVideoSortViews, FolderVideoSortLikes:
		return sort
	default:
		return FolderVideoSortLatest
	}
}

// TrimKeyword 裁剪搜索关键字两端空白（列表查询共用）。
func TrimKeyword(keyword string) string {
	return strings.TrimSpace(keyword)
}

// trimSpace 领域内部使用的空白裁剪助手。
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}
