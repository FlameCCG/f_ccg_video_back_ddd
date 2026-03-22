package video

import (
	"context"
	"errors"
	"testing"

	videodomain "f_ccg_video_back_ddd/internal/domain/video"
)

// folderRepoStub 收藏夹仓储桩（仅覆盖 UpdateFolder 用例路径）。
type folderRepoStub struct {
	folders   map[uint]*videodomain.FavoriteFolder // id → folder（FindByIDAndUser 校验 user_id）
	updateErr error                                // Update 注入的存储错误
}

func (s *folderRepoStub) FindDefault(context.Context, uint) (*videodomain.FavoriteFolder, error) {
	return nil, nil
}

func (s *folderRepoStub) Create(_ context.Context, folder *videodomain.FavoriteFolder) (*videodomain.FavoriteFolder, error) {
	return folder, nil
}

func (s *folderRepoStub) FindByIDAndUser(_ context.Context, userID, folderID uint) (*videodomain.FavoriteFolder, error) {
	folder, ok := s.folders[folderID]
	if !ok || folder.UserID() != userID {
		return nil, videodomain.ErrRecordNotFound
	}

	return folder, nil
}

func (s *folderRepoStub) Update(_ context.Context, _ *videodomain.FavoriteFolder) error {
	return s.updateErr
}

func (s *folderRepoStub) DeleteWithVideos(context.Context, uint, *videodomain.FavoriteFolder) ([]uint, error) {
	return nil, nil
}

func (s *folderRepoStub) LinkExists(context.Context, uint, uint) (bool, error) {
	return false, nil
}

func (s *folderRepoStub) DeleteLink(context.Context, uint, uint) error {
	return nil
}

func (s *folderRepoStub) EnsureLink(context.Context, uint, uint) (bool, error) {
	return false, nil
}

func (s *folderRepoStub) UserFavorited(context.Context, uint, uint) (bool, error) {
	return false, nil
}

// folderListCacheStub 收藏夹列表缓存桩，记录失效调用。
type folderListCacheStub struct {
	invalidated uint
}

func (s *folderListCacheStub) Load(uint, uint) ([]FolderWithCount, bool) {
	return nil, false
}

func (s *folderListCacheStub) Store(uint, uint, []FolderWithCount) {}

func (s *folderListCacheStub) Invalidate(userID uint) {
	s.invalidated = userID
}

// newFolderServiceForTest 构造收藏夹用例服务（UpdateFolder 不触碰 query/stats/authors/creators，传 nil）。
func newFolderServiceForTest(folders *folderRepoStub, cache *folderListCacheStub) *FolderService {
	return NewFolderService(nil, folders, nil, cache, nil, nil, nil)
}

func TestUpdateFolderSuccess(t *testing.T) {
	t.Parallel()
	folders := &folderRepoStub{folders: map[uint]*videodomain.FavoriteFolder{
		1: videodomain.RestoreFavoriteFolder(videodomain.FavoriteFolderSnapshot{ID: 1, UserID: 7, Name: "旧名字", IsDefault: true}),
	}}
	cache := &folderListCacheStub{}

	folder, err := newFolderServiceForTest(folders, cache).UpdateFolder(t.Context(), 7, 1, "新名字")
	if err != nil {
		t.Fatalf("UpdateFolder: %v", err)
	}

	if folder.Name() != "新名字" {
		t.Fatalf("name = %q, want %q", folder.Name(), "新名字")
	}

	if folder.UserID() != 7 || !folder.IsDefault() {
		t.Fatalf("unrelated fields mutated: %+v", folder)
	}

	if cache.invalidated != 7 {
		t.Fatalf("cache invalidated user = %d, want 7", cache.invalidated)
	}
}

func TestUpdateFolderUserNotLoggedIn(t *testing.T) {
	t.Parallel()
	_, err := newFolderServiceForTest(&folderRepoStub{}, &folderListCacheStub{}).UpdateFolder(t.Context(), 0, 1, "名字")
	if !errors.Is(err, videodomain.ErrUserNotLogin) {
		t.Fatalf("err = %v, want %v", err, videodomain.ErrUserNotLogin)
	}
}

func TestUpdateFolderNotFoundOrForbidden(t *testing.T) {
	t.Parallel()
	folders := &folderRepoStub{folders: map[uint]*videodomain.FavoriteFolder{
		1: videodomain.RestoreFavoriteFolder(videodomain.FavoriteFolderSnapshot{ID: 1, UserID: 7, Name: "别人的夹子"}),
	}}

	_, err := newFolderServiceForTest(folders, &folderListCacheStub{}).UpdateFolder(t.Context(), 7, 99, "名字")
	if !errors.Is(err, videodomain.ErrFolderNotFoundOrForbidden) {
		t.Fatalf("err = %v, want %v", err, videodomain.ErrFolderNotFoundOrForbidden)
	}

	_, err = newFolderServiceForTest(folders, &folderListCacheStub{}).UpdateFolder(t.Context(), 8, 1, "名字")
	if !errors.Is(err, videodomain.ErrFolderNotFoundOrForbidden) {
		t.Fatalf("other user's folder err = %v, want %v", err, videodomain.ErrFolderNotFoundOrForbidden)
	}
}

func TestUpdateFolderEmptyName(t *testing.T) {
	t.Parallel()
	folders := &folderRepoStub{folders: map[uint]*videodomain.FavoriteFolder{
		1: videodomain.RestoreFavoriteFolder(videodomain.FavoriteFolderSnapshot{ID: 1, UserID: 7, Name: "旧名字"}),
	}}

	_, err := newFolderServiceForTest(folders, &folderListCacheStub{}).UpdateFolder(t.Context(), 7, 1, "")
	if !errors.Is(err, videodomain.ErrFolderNameEmpty) {
		t.Fatalf("err = %v, want %v", err, videodomain.ErrFolderNameEmpty)
	}
}

func TestUpdateFolderPropagatesRepoError(t *testing.T) {
	t.Parallel()
	updateErr := errors.New("db unavailable")
	folders := &folderRepoStub{
		folders: map[uint]*videodomain.FavoriteFolder{
			1: videodomain.RestoreFavoriteFolder(videodomain.FavoriteFolderSnapshot{ID: 1, UserID: 7, Name: "旧名字"}),
		},
		updateErr: updateErr,
	}

	_, err := newFolderServiceForTest(folders, &folderListCacheStub{}).UpdateFolder(t.Context(), 7, 1, "新名字")
	if !errors.Is(err, updateErr) {
		t.Fatalf("err = %v, want %v", err, updateErr)
	}
}
