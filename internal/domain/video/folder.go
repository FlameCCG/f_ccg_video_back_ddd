package video

// DefaultFolderName 默认收藏夹名称。
const DefaultFolderName = "默认收藏夹"

// FavoriteFolder 收藏夹实体。
type FavoriteFolder struct {
	id        uint
	userID    uint
	name      string
	isDefault bool
}

// FavoriteFolderSnapshot 是收藏夹的持久化快照。
type FavoriteFolderSnapshot struct {
	ID        uint
	UserID    uint
	Name      string
	IsDefault bool
}

// NewFavoriteFolder 校验收藏夹名称后创建非默认收藏夹。
func NewFavoriteFolder(userID uint, name string) (*FavoriteFolder, error) {
	if err := (FavoriteFolder{}).ValidateName(name); err != nil {
		return nil, err
	}

	return &FavoriteFolder{userID: userID, name: name}, nil
}

// NewDefaultFavoriteFolder 创建默认收藏夹。名称使用既有常量，不额外收紧规则。
func NewDefaultFavoriteFolder(userID uint) *FavoriteFolder {
	return &FavoriteFolder{userID: userID, name: DefaultFolderName, isDefault: true}
}

// RestoreFavoriteFolder 从快照恢复收藏夹，不重新校验名称。
func RestoreFavoriteFolder(snapshot FavoriteFolderSnapshot) *FavoriteFolder {
	return &FavoriteFolder{
		id:        snapshot.ID,
		userID:    snapshot.UserID,
		name:      snapshot.Name,
		isDefault: snapshot.IsDefault,
	}
}

// Snapshot 返回收藏夹快照。
func (f *FavoriteFolder) Snapshot() FavoriteFolderSnapshot {
	if f == nil {
		return FavoriteFolderSnapshot{}
	}

	return FavoriteFolderSnapshot{ID: f.id, UserID: f.userID, Name: f.name, IsDefault: f.isDefault}
}

func (f *FavoriteFolder) ID() uint {
	if f == nil {
		return 0
	}

	return f.id
}

func (f *FavoriteFolder) UserID() uint {
	if f == nil {
		return 0
	}

	return f.userID
}

func (f *FavoriteFolder) Name() string {
	if f == nil {
		return ""
	}

	return f.name
}

func (f *FavoriteFolder) IsDefault() bool {
	return f != nil && f.isDefault
}

// CanDelete 判断收藏夹是否允许删除（默认收藏夹不可删除）。
func (f *FavoriteFolder) CanDelete() error {
	if f == nil {
		return ErrRecordNotFound
	}

	if f.isDefault {
		return ErrDefaultFolderUndeletable
	}

	return nil
}

// Rename 重命名收藏夹：校验新名称后更新实体。失败时保持原名称。
func (f *FavoriteFolder) Rename(name string) error {
	if f == nil {
		return ErrRecordNotFound
	}

	if err := f.ValidateName(name); err != nil {
		return err
	}

	f.name = name

	return nil
}

// ValidateName 校验收藏夹名称，只判断空串，不做 TrimSpace。
func (FavoriteFolder) ValidateName(name string) error {
	if name == "" {
		return ErrFolderNameEmpty
	}

	return nil
}

// VideoSort 归一化收藏夹内视频排序值，非法值回落最近收藏。
func (FavoriteFolder) VideoSort(sort int) int {
	switch sort {
	case FolderVideoSortViews, FolderVideoSortLikes:
		return sort
	default:
		return FolderVideoSortLatest
	}
}
