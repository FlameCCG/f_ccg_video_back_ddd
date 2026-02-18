package contentops

import (
	"context"
	"errors"
	"testing"

	domain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
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
	t.Run("非首页类型不允许按分区筛选", func(t *testing.T) {
		svc, _, _, _ := newBannerServiceFixture()

		partitionID := uint(3)

		_, err := svc.ListVisible(context.Background(), VisibleBannerParams{Type: 2, PartitionID: &partitionID})
		if !errors.Is(err, ErrBannerPartitionFilterUnsupported) {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("只带分区时按首页类型查询", func(t *testing.T) {
		svc, repo, _, _ := newBannerServiceFixture()

		partitionID := uint(3)

		if _, err := svc.ListVisible(context.Background(), VisibleBannerParams{PartitionID: &partitionID}); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if repo.visibleType != enum.BannerTypeHome {
			t.Fatalf("unexpected banner type: %d", repo.visibleType)
		}

		if repo.visiblePartition == nil || *repo.visiblePartition != 3 {
			t.Fatalf("unexpected partition: %v", repo.visiblePartition)
		}
	})

	t.Run("首页类型未带分区时回落首页分区0", func(t *testing.T) {
		svc, repo, _, _ := newBannerServiceFixture()

		if _, err := svc.ListVisible(context.Background(), VisibleBannerParams{Type: 1}); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if repo.visiblePartition == nil || *repo.visiblePartition != 0 {
			t.Fatalf("unexpected partition: %v", repo.visiblePartition)
		}
	})

	t.Run("其它类型不带分区条件", func(t *testing.T) {
		svc, repo, _, _ := newBannerServiceFixture()

		if _, err := svc.ListVisible(context.Background(), VisibleBannerParams{Type: 3}); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if repo.visiblePartition != nil {
			t.Fatalf("unexpected partition: %v", *repo.visiblePartition)
		}

		if repo.visibleType != enum.BannerTypeUser {
			t.Fatalf("unexpected banner type: %d", repo.visibleType)
		}
	})
}

func TestBannerServiceListVisibleUsesCache(t *testing.T) {
	svc, repo, cache, _ := newBannerServiceFixture()
	cache.hit = []domain.Banner{{ID: 9}}
	cache.hitOK = true

	list, err := svc.ListVisible(context.Background(), VisibleBannerParams{Type: 1})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if len(list) != 1 || list[0].ID != 9 {
		t.Fatalf("cache hit should short-circuit repo, got %+v", list)
	}

	if repo.visibleType != 0 {
		t.Fatal("repo should not be queried on cache hit")
	}

	// 未命中时回填缓存
	cache.hitOK = false
	repo.visible = []domain.Banner{{ID: 1}}

	if _, err := svc.ListVisible(context.Background(), VisibleBannerParams{Type: 1}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if len(cache.stored) != 1 || cache.storedType != enum.BannerTypeHome {
		t.Fatalf("unexpected stored cache: type=%d items=%+v", cache.storedType, cache.stored)
	}
}

func TestBannerServiceCreateResolvesPartition(t *testing.T) {
	t.Run("非首页类型携带分区报错", func(t *testing.T) {
		svc, _, _, _ := newBannerServiceFixture()

		partitionID := uint(2)

		_, err := svc.Create(context.Background(), CreateBannerParams{Cover: "c", Type: 2, PartitionID: &partitionID})

		var resolveErr *PartitionResolveError
		if !errors.As(err, &resolveErr) || resolveErr.Error() != "只有类型1支持设置分区ID" {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("分区不存在报错", func(t *testing.T) {
		svc, _, _, partitions := newBannerServiceFixture()
		partitions.exists = false

		partitionID := uint(9)

		_, err := svc.Create(context.Background(), CreateBannerParams{Cover: "c", Type: 1, PartitionID: &partitionID})

		var resolveErr *PartitionResolveError
		if !errors.As(err, &resolveErr) || resolveErr.Error() != "分区不存在" {
			t.Fatalf("unexpected err: %v", err)
		}

		if len(partitions.asked) != 1 || partitions.asked[0] != 9 {
			t.Fatalf("unexpected partition lookups: %v", partitions.asked)
		}
	})

	t.Run("默认显示且默认首页类型", func(t *testing.T) {
		svc, repo, cache, partitions := newBannerServiceFixture()

		banner, err := svc.Create(context.Background(), CreateBannerParams{Cover: "c"})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if !banner.Show || banner.Type != enum.BannerTypeHome || banner.PartitionID != 0 {
			t.Fatalf("unexpected banner: %+v", banner)
		}

		if banner.ID != 100 || repo.created == nil {
			t.Fatalf("create should persist and backfill id, got %+v", banner)
		}

		if len(partitions.asked) != 0 {
			t.Fatalf("首页分区 0 无需校验存在性, got %v", partitions.asked)
		}

		if cache.invalidCount != 1 {
			t.Fatalf("create should invalidate cache once, got %d", cache.invalidCount)
		}
	})
}

func TestBannerServiceUpdate(t *testing.T) {
	t.Run("无可更新字段", func(t *testing.T) {
		svc, _, _, _ := newBannerServiceFixture()

		err := svc.Update(context.Background(), UpdateBannerParams{ID: 1})
		if !errors.Is(err, domain.ErrBannerNoUpdatableField) {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("记录不存在", func(t *testing.T) {
		svc, _, _, _ := newBannerServiceFixture()

		bannerType := 2

		err := svc.Update(context.Background(), UpdateBannerParams{ID: 1, Type: &bannerType})
		if !errors.Is(err, ErrBannerNotFound) {
			t.Fatalf("unexpected err: %v", err)
		}
	})

	t.Run("改为非首页类型时分区归零", func(t *testing.T) {
		svc, repo, cache, _ := newBannerServiceFixture()
		repo.byID[1] = &domain.Banner{ID: 1, Type: enum.BannerTypeHome, PartitionID: 6}

		bannerType := 2
		if err := svc.Update(context.Background(), UpdateBannerParams{ID: 1, Type: &bannerType}); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if repo.updatedID != 1 {
			t.Fatalf("unexpected updated id: %d", repo.updatedID)
		}

		if repo.updatedChanges.Type == nil || *repo.updatedChanges.Type != enum.BannerTypeTop {
			t.Fatalf("unexpected type change: %+v", repo.updatedChanges.Type)
		}

		if repo.updatedChanges.PartitionID == nil || *repo.updatedChanges.PartitionID != 0 {
			t.Fatalf("unexpected partition change: %+v", repo.updatedChanges.PartitionID)
		}

		if cache.invalidCount != 1 {
			t.Fatalf("update should invalidate cache once, got %d", cache.invalidCount)
		}
	})

	t.Run("仅改封面时不触碰分区", func(t *testing.T) {
		svc, repo, _, _ := newBannerServiceFixture()
		repo.byID[1] = &domain.Banner{ID: 1, Type: enum.BannerTypeHome, PartitionID: 6}

		cover := "new"
		if err := svc.Update(context.Background(), UpdateBannerParams{ID: 1, Cover: &cover}); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}

		if repo.updatedChanges.PartitionID != nil || repo.updatedChanges.Type != nil {
			t.Fatalf("unexpected changes: %+v", repo.updatedChanges)
		}

		if repo.updatedChanges.Cover == nil || *repo.updatedChanges.Cover != "new" {
			t.Fatalf("unexpected cover change: %+v", repo.updatedChanges.Cover)
		}
	})
}

func TestBannerServiceDeleteEmptyIDsIsNoop(t *testing.T) {
	svc, repo, cache, _ := newBannerServiceFixture()

	if err := svc.Delete(context.Background(), nil); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if repo.deletedIDs != nil {
		t.Fatalf("repo should not be called, got %v", repo.deletedIDs)
	}

	if cache.invalidCount != 0 {
		t.Fatalf("empty delete should not invalidate cache, got %d", cache.invalidCount)
	}

	if err := svc.Delete(context.Background(), []uint{1, 2}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if len(repo.deletedIDs) != 2 || cache.invalidCount != 1 {
		t.Fatalf("unexpected delete result: ids=%v invalidations=%d", repo.deletedIDs, cache.invalidCount)
	}
}

func TestBannerServiceListAdminNormalizesPaging(t *testing.T) {
	svc, repo, _, _ := newBannerServiceFixture()

	partitionID := uint(4)

	if _, _, err := svc.ListAdmin(context.Background(), AdminBannerListParams{PartitionID: &partitionID}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if repo.adminQuery.Page != 1 || repo.adminQuery.PageSize != 20 {
		t.Fatalf("unexpected paging: %+v", repo.adminQuery)
	}

	// 只带分区时按首页类型过滤（与前台一致）
	if repo.adminQuery.Type != enum.BannerTypeHome {
		t.Fatalf("unexpected banner type: %d", repo.adminQuery.Type)
	}
}
