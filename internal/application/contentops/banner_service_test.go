package contentops

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"testing"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/contentops"
)

// fakeBannerRepo 记录调用参数的轮播图仓储替身（测试不走 wire）。
type fakeBannerRepo struct {
	visible          []domain.Banner
	visibleType      enum.BannerType
	visiblePartition *uint
	adminQuery       domain.BannerAdminQuery
	byID             map[uint]*domain.Banner
	created          *domain.Banner
	updatedID        uint
	updatedChanges   domain.BannerChanges
	deletedIDs       []uint
	err              error
}

func (r *fakeBannerRepo) ListVisible(
	_ context.Context,
	bannerType enum.BannerType,
	partitionID *uint,
) ([]domain.Banner, error) {
	r.visibleType = bannerType
	r.visiblePartition = partitionID

	return r.visible, r.err
}

func (r *fakeBannerRepo) ListAdmin(
	_ context.Context,
	query domain.BannerAdminQuery,
) ([]domain.Banner, int64, error) {
	r.adminQuery = query

	return r.visible, int64(len(r.visible)), r.err
}

func (r *fakeBannerRepo) ListVisibleByIDs(
	_ context.Context,
	_ enum.BannerType,
	ids []uint,
) ([]domain.Banner, error) {
	out := make([]domain.Banner, 0, len(ids))
	for _, id := range ids {
		if banner, ok := r.byID[id]; ok {
			out = append(out, *banner)
		}
	}

	return out, r.err
}

func (r *fakeBannerRepo) FindByID(_ context.Context, bannerID uint) (*domain.Banner, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.byID[bannerID], nil
}

func (r *fakeBannerRepo) Create(_ context.Context, banner *domain.Banner) error {
	if r.err != nil {
		return r.err
	}

	banner.ID = 100
	r.created = banner

	return nil
}

func (r *fakeBannerRepo) Update(_ context.Context, bannerID uint, changes domain.BannerChanges) error {
	r.updatedID = bannerID
	r.updatedChanges = changes

	return r.err
}

func (r *fakeBannerRepo) Delete(_ context.Context, ids []uint) error {
	r.deletedIDs = ids

	return r.err
}

// fakeBannerCache 记录缓存命中与失效次数。
type fakeBannerCache struct {
	hit          []domain.Banner
	hitOK        bool
	storedType   enum.BannerType
	stored       []domain.Banner
	invalidCount int
}

func (c *fakeBannerCache) LoadVisible(_ enum.BannerType, _ *uint) ([]domain.Banner, bool) {
	return c.hit, c.hitOK
}

func (c *fakeBannerCache) StoreVisible(bannerType enum.BannerType, _ *uint, items []domain.Banner) {
	c.storedType = bannerType
	c.stored = items
}

func (c *fakeBannerCache) InvalidateVisible() {
	c.invalidCount++
}

// fakePartitionQuery 固定的分区存在性判定。
type fakePartitionQuery struct {
	exists bool
	err    error
	asked  []uint
}

func (q *fakePartitionQuery) Exists(_ context.Context, partitionID uint) (bool, error) {
	q.asked = append(q.asked, partitionID)

	return q.exists, q.err
}

func newBannerServiceFixture() (*BannerService, *fakeBannerRepo, *fakeBannerCache, *fakePartitionQuery) {
	repo := &fakeBannerRepo{byID: map[uint]*domain.Banner{}}
	cache := &fakeBannerCache{}
	partitions := &fakePartitionQuery{exists: true}

	return NewBannerService(repo, cache, partitions), repo, cache, partitions
}

func TestBannerServiceListVisibleNormalizesFilters(t *testing.T) {
	t.Parallel()

	t.Run("非首页类型不允许按分区筛选", func(t *testing.T) {
		t.Parallel()

		svc, _, _, _ := newBannerServiceFixture()

		partitionID := uint(3)

		_, err := svc.ListVisible(context.Background(), VisibleBannerParams{Type: 2, PartitionID: &partitionID})
		require.ErrorIs(t, err, ErrBannerPartitionFilterUnsupported, "unexpected err: %v", err)
	})

	t.Run("只带分区时按首页类型查询", func(t *testing.T) {
		t.Parallel()

		svc, repo, _, _ := newBannerServiceFixture()

		partitionID := uint(3)

		{
			_, err := svc.ListVisible(context.Background(), VisibleBannerParams{PartitionID: &partitionID})
			require.NoError(t, err, "unexpected err: %v", err)
		}

		require.Equal(t, enum.BannerTypeHome, repo.visibleType, "unexpected banner type: %d", repo.visibleType)

		require.NotNil(t, repo.visiblePartition, "unexpected partition: %v", repo.visiblePartition)
		require.EqualValues(t, 3, *repo.visiblePartition, "unexpected partition: %v", repo.visiblePartition)
	})

	t.Run("首页类型未带分区时回落首页分区0", func(t *testing.T) {
		t.Parallel()

		svc, repo, _, _ := newBannerServiceFixture()

		{
			_, err := svc.ListVisible(context.Background(), VisibleBannerParams{Type: 1})
			require.NoError(t, err, "unexpected err: %v", err)
		}

		require.NotNil(t, repo.visiblePartition, "unexpected partition: %v", repo.visiblePartition)
		require.EqualValues(t, 0, *repo.visiblePartition, "unexpected partition: %v", repo.visiblePartition)
	})

	t.Run("其它类型不带分区条件", func(t *testing.T) {
		t.Parallel()

		svc, repo, _, _ := newBannerServiceFixture()

		{
			_, err := svc.ListVisible(context.Background(), VisibleBannerParams{Type: 3})
			require.NoError(t, err, "unexpected err: %v", err)
		}

		require.Nil(t, repo.visiblePartition, "unexpected partition: %v", repo.visiblePartition)

		require.Equal(t, enum.BannerTypeUser, repo.visibleType, "unexpected banner type: %d", repo.visibleType)
	})
}

func TestBannerServiceListVisibleUsesCache(t *testing.T) {
	t.Parallel()

	svc, repo, cache, _ := newBannerServiceFixture()
	cache.hit = []domain.Banner{{ID: 9}}
	cache.hitOK = true

	list, err := svc.ListVisible(context.Background(), VisibleBannerParams{Type: 1})
	require.NoError(t, err, "unexpected err: %v", err)

	require.Len(t, list, 1, "cache hit should short-circuit repo, got %+v", list)
	require.EqualValues(t, 9, list[0].ID, "cache hit should short-circuit repo, got %+v", list)

	require.EqualValues(t, 0, repo.visibleType, "repo should not be queried on cache hit")

	// 未命中时回填缓存
	cache.hitOK = false
	repo.visible = []domain.Banner{{ID: 1}}

	{
		_, err := svc.ListVisible(context.Background(), VisibleBannerParams{Type: 1})
		require.NoError(t, err, "unexpected err: %v", err)
	}

	require.Len(t, cache.stored, 1, "unexpected stored cache: type=%d items=%+v", cache.storedType, cache.stored)
	require.Equal(t, enum.BannerTypeHome, cache.storedType, "unexpected stored cache: type=%d items=%+v", cache.storedType, cache.stored)
}

func TestBannerServiceCreateResolvesPartition(t *testing.T) {
	t.Parallel()

	t.Run("非首页类型携带分区报错", func(t *testing.T) {
		t.Parallel()

		svc, _, _, _ := newBannerServiceFixture()

		partitionID := uint(2)

		_, err := svc.Create(context.Background(), CreateBannerParams{Cover: "c", Type: 2, PartitionID: &partitionID})

		var resolveErr *PartitionResolveError
		require.ErrorAs(t, err, &resolveErr, "unexpected err: %v", err)
		require.Equal(t, "只有类型1支持设置分区ID", resolveErr.Error(), "unexpected err: %v", err)
	})

	t.Run("分区不存在报错", func(t *testing.T) {
		t.Parallel()

		svc, _, _, partitions := newBannerServiceFixture()
		partitions.exists = false

		partitionID := uint(9)

		_, err := svc.Create(context.Background(), CreateBannerParams{Cover: "c", Type: 1, PartitionID: &partitionID})

		var resolveErr *PartitionResolveError
		require.ErrorAs(t, err, &resolveErr, "unexpected err: %v", err)
		require.Equal(t, "分区不存在", resolveErr.Error(), "unexpected err: %v", err)

		require.Len(t, partitions.asked, 1, "unexpected partition lookups: %v", partitions.asked)
		require.EqualValues(t, 9, partitions.asked[0], "unexpected partition lookups: %v", partitions.asked)
	})

	t.Run("默认显示且默认首页类型", func(t *testing.T) {
		t.Parallel()

		svc, repo, cache, partitions := newBannerServiceFixture()

		banner, err := svc.Create(context.Background(), CreateBannerParams{Cover: "c"})
		require.NoError(t, err, "unexpected err: %v", err)

		require.True(t, banner.Show, "unexpected banner: %+v", banner)
		require.Equal(t, enum.BannerTypeHome, banner.Type, "unexpected banner: %+v", banner)
		require.EqualValues(t, 0, banner.PartitionID, "unexpected banner: %+v", banner)

		require.EqualValues(t, 100, banner.ID, "create should persist and backfill id, got %+v", banner)
		require.NotNil(t, repo.created, "create should persist and backfill id, got %+v", banner)

		require.Empty(t, partitions.asked, "首页分区 0 无需校验存在性, got %v", partitions.asked)

		require.Equal(t, 1, cache.invalidCount, "create should invalidate cache once, got %d", cache.invalidCount)
	})
}

func TestBannerServiceUpdate(t *testing.T) {
	t.Parallel()

	t.Run("无可更新字段", func(t *testing.T) {
		t.Parallel()

		svc, _, _, _ := newBannerServiceFixture()

		err := svc.Update(context.Background(), UpdateBannerParams{ID: 1})
		require.ErrorIs(t, err, domain.ErrBannerNoUpdatableField, "unexpected err: %v", err)
	})

	t.Run("记录不存在", func(t *testing.T) {
		t.Parallel()

		svc, _, _, _ := newBannerServiceFixture()

		bannerType := 2

		err := svc.Update(context.Background(), UpdateBannerParams{ID: 1, Type: &bannerType})
		require.ErrorIs(t, err, ErrBannerNotFound, "unexpected err: %v", err)
	})

	t.Run("改为非首页类型时分区归零", func(t *testing.T) {
		t.Parallel()

		svc, repo, cache, _ := newBannerServiceFixture()
		repo.byID[1] = &domain.Banner{ID: 1, Type: enum.BannerTypeHome, PartitionID: 6}

		bannerType := 2
		{
			err := svc.Update(context.Background(), UpdateBannerParams{ID: 1, Type: &bannerType})
			require.NoError(t, err, "unexpected err: %v", err)
		}

		require.EqualValues(t, 1, repo.updatedID, "unexpected updated id: %d", repo.updatedID)

		require.NotNil(t, repo.updatedChanges.Type, "unexpected type change: %+v", repo.updatedChanges.Type)
		require.Equal(t, enum.BannerTypeTop, *repo.updatedChanges.Type, "unexpected type change: %+v", repo.updatedChanges.Type)

		require.NotNil(t, repo.updatedChanges.PartitionID, "unexpected partition change: %+v", repo.updatedChanges.PartitionID)
		require.EqualValues(t, 0, *repo.updatedChanges.PartitionID, "unexpected partition change: %+v", repo.updatedChanges.PartitionID)

		require.Equal(t, 1, cache.invalidCount, "update should invalidate cache once, got %d", cache.invalidCount)
	})

	t.Run("仅改封面时不触碰分区", func(t *testing.T) {
		t.Parallel()

		svc, repo, _, _ := newBannerServiceFixture()
		repo.byID[1] = &domain.Banner{ID: 1, Type: enum.BannerTypeHome, PartitionID: 6}

		cover := "new"
		{
			err := svc.Update(context.Background(), UpdateBannerParams{ID: 1, Cover: &cover})
			require.NoError(t, err, "unexpected err: %v", err)
		}

		require.Nil(t, repo.updatedChanges.PartitionID, "unexpected changes: %+v", repo.updatedChanges)
		require.Nil(t, repo.updatedChanges.Type, "unexpected changes: %+v", repo.updatedChanges)

		require.NotNil(t, repo.updatedChanges.Cover, "unexpected cover change: %+v", repo.updatedChanges.Cover)
		require.Equal(t, "new", *repo.updatedChanges.Cover, "unexpected cover change: %+v", repo.updatedChanges.Cover)
	})
}

func TestBannerServiceDeleteEmptyIDsIsNoop(t *testing.T) {
	t.Parallel()

	svc, repo, cache, _ := newBannerServiceFixture()

	{
		err := svc.Delete(context.Background(), nil)
		require.NoError(t, err, "unexpected err: %v", err)
	}

	require.Nil(t, repo.deletedIDs, "repo should not be called, got %v", repo.deletedIDs)

	require.Equal(t, 0, cache.invalidCount, "empty delete should not invalidate cache, got %d", cache.invalidCount)

	{
		err := svc.Delete(context.Background(), []uint{1, 2})
		require.NoError(t, err, "unexpected err: %v", err)
	}

	require.Len(t, repo.deletedIDs, 2, "unexpected delete result: ids=%v invalidations=%d", repo.deletedIDs, cache.invalidCount)
	require.Equal(t, 1, cache.invalidCount, "unexpected delete result: ids=%v invalidations=%d", repo.deletedIDs, cache.invalidCount)
}

func TestBannerServiceListAdminNormalizesPaging(t *testing.T) {
	t.Parallel()

	svc, repo, _, _ := newBannerServiceFixture()

	partitionID := uint(4)

	{
		_, _, err := svc.ListAdmin(context.Background(), AdminBannerListParams{PartitionID: &partitionID})
		require.NoError(t, err, "unexpected err: %v", err)
	}

	require.Equal(t, 1, repo.adminQuery.Page, "unexpected paging: %+v", repo.adminQuery)
	require.Equal(t, 20, repo.adminQuery.PageSize, "unexpected paging: %+v", repo.adminQuery)

	// 只带分区时按首页类型过滤（与前台一致）
	require.Equal(t, enum.BannerTypeHome, repo.adminQuery.Type, "unexpected banner type: %d", repo.adminQuery.Type)
}
