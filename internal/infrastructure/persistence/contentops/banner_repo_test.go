package contentops

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/contentops"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newBannerFixture 用 sqlite 临时库装配轮播图仓储与分区查询（测试不走 wire）。
func newBannerFixture(t *testing.T) (*BannerRepository, *VideoPartitionQuery, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "banner-test.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "open sqlite failed: %v", err)

	{
		err := db.AutoMigrate(&po.Banner{}, &po.VideoPartition{})
		require.NoError(t, err, "migrate failed: %v", err)
	}

	sqlDB, err := db.DB()
	require.NoError(t, err, "db.DB: %v", err)

	t.Cleanup(func() { _ = sqlDB.Close() })

	return NewBannerRepository(db), NewVideoPartitionQuery(db), db
}

// seedBanner 落库一条轮播图并按需修正 show 列。
//
// po.Banner.Show 带 `default:true` tag，GORM 会在插入时省略 false 零值并回填数据库默认值，
// 因此隐藏项必须显式再 Update 一次（该行为对原实现同样存在，见 report 中的 FIND 条目）。
func seedBanner(t *testing.T, db *gorm.DB, row *po.Banner, show bool) {
	t.Helper()

	row.Show = show
	{
		err := db.Create(row).Error
		require.NoError(t, err, "seed banner failed: %v", err)
	}

	{
		err := db.Model(&po.Banner{}).Where("id = ?", row.ID).Update("show", show).Error
		require.NoError(t, err, "seed banner show failed: %v", err)
	}

	row.Show = show
}

func TestBannerRepositoryCreateAndRead(t *testing.T) {
	t.Parallel()

	repo, _, _ := newBannerFixture(t)
	ctx := context.Background()

	banner := &domain.Banner{
		Cover:       "https://example.com/a.png",
		Href:        "https://example.com",
		Show:        true,
		Type:        enum.BannerTypeHome,
		PartitionID: 0,
	}
	{
		err := repo.Create(ctx, banner)
		require.NoError(t, err, "create banner failed: %v", err)
	}

	require.NotEqualValues(t, 0, banner.ID, "create should backfill auto increment id")

	require.False(t, banner.CreatedAt.IsZero(), "create should backfill created_at")

	found, err := repo.FindByID(ctx, banner.ID)
	require.NoError(t, err, "find banner failed: %v", err)

	require.NotNil(t, found, "unexpected banner: %+v", found)
	require.Equal(t, found.Cover, banner.Cover, "unexpected banner: %+v", found)
	require.Equal(t, enum.BannerTypeHome, found.Type, "unexpected banner: %+v", found)

	missing, err := repo.FindByID(ctx, banner.ID+999)
	require.NoError(t, err, "find missing banner should not error: %v", err)

	require.Nil(t, missing, "missing banner should be nil, got %+v", missing)
}

func TestBannerRepositoryListVisibleFiltersAndOrder(t *testing.T) {
	t.Parallel()

	repo, _, db := newBannerFixture(t)
	ctx := context.Background()

	rows := []po.Banner{
		{Cover: "home-1", Type: enum.BannerTypeHome, PartitionID: 0},
		{Cover: "home-2", Type: enum.BannerTypeHome, PartitionID: 0},
		{Cover: "home-partition", Type: enum.BannerTypeHome, PartitionID: 7},
		{Cover: "top", Type: enum.BannerTypeTop, PartitionID: 0},
		{Cover: "hidden", Type: enum.BannerTypeHome, PartitionID: 0},
	}
	for i := range rows {
		seedBanner(t, db, &rows[i], rows[i].Cover != "hidden")
	}

	homePartition := uint(0)

	list, err := repo.ListVisible(ctx, enum.BannerTypeHome, &homePartition)
	require.NoError(t, err, "list visible failed: %v", err)

	require.Len(t, list, 2, "expected 2 visible home banners, got %d (%+v)", len(list), list)

	// id DESC 排序
	require.Equal(t, "home-2", list[0].Cover, "expected id desc order, got %+v", list)
	require.Equal(t, "home-1", list[1].Cover, "expected id desc order, got %+v", list)

	// 不限类型、不限分区时返回全部可见项
	all, err := repo.ListVisible(ctx, 0, nil)
	require.NoError(t, err, "list visible all failed: %v", err)

	require.Len(t, all, 4, "expected 4 visible banners, got %d", len(all))

	// 无匹配数据：GORM 会把目标切片初始化为非 nil 空切片，
	// 因此响应体序列化为 []（与原实现一致，不是 null）。
	empty, err := repo.ListVisible(ctx, enum.BannerTypeUser, nil)
	require.NoError(t, err, "list visible empty failed: %v", err)

	require.NotNil(t, empty, "expected empty non-nil slice, got %+v", empty)
	require.Empty(t, empty, "expected empty non-nil slice, got %+v", empty)
}

func TestBannerRepositoryListAdminPaging(t *testing.T) {
	t.Parallel()

	repo, _, db := newBannerFixture(t)
	ctx := context.Background()

	for i := range 5 {
		row := po.Banner{Cover: "cover", Type: enum.BannerTypeHome, PartitionID: uint(i % 2)}
		seedBanner(t, db, &row, true)
	}

	query := domain.BannerAdminQuery{Page: 1, PageSize: 2, Type: enum.BannerTypeHome}
	query.Normalize()

	list, total, err := repo.ListAdmin(ctx, query)
	require.NoError(t, err, "list admin failed: %v", err)

	require.EqualValues(t, 5, total, "unexpected total: %d", total)

	require.Len(t, list, 2, "unexpected page size: %d", len(list))

	partitionID := uint(1)
	filtered := domain.BannerAdminQuery{PartitionID: &partitionID}
	filtered.Normalize()

	list, total, err = repo.ListAdmin(ctx, filtered)
	require.NoError(t, err, "list admin by partition failed: %v", err)

	require.False(t, total != 2 || len(list) != 2, "unexpected filtered result: total=%d len=%d", total, len(list))

	missingPartition := uint(99)
	none := domain.BannerAdminQuery{PartitionID: &missingPartition}
	none.Normalize()

	list, total, err = repo.ListAdmin(ctx, none)
	require.NoError(t, err, "list admin empty failed: %v", err)

	require.EqualValues(t, 0, total, "empty admin list should be an empty non-nil slice, got total=%d list=%+v", total, list)
	require.NotNil(t, list, "empty admin list should be an empty non-nil slice, got total=%d list=%+v", total, list)
	require.Empty(t, list, "empty admin list should be an empty non-nil slice, got total=%d list=%+v", total, list)
}

func TestBannerRepositoryListVisibleByIDsKeepsRequestedOrder(t *testing.T) {
	t.Parallel()

	repo, _, db := newBannerFixture(t)
	ctx := context.Background()

	ids := make([]uint, 0, 3)

	for _, cover := range []string{"a", "b", "c"} {
		row := po.Banner{Cover: cover, Type: enum.BannerTypeUser}
		seedBanner(t, db, &row, true)

		ids = append(ids, row.ID)
	}

	requested := []uint{ids[2], ids[0], ids[1] + 1000, ids[1]}

	list, err := repo.ListVisibleByIDs(ctx, enum.BannerTypeUser, requested)
	require.NoError(t, err, "list visible by ids failed: %v", err)

	require.Len(t, list, 3, "expected 3 banners (missing id skipped), got %d", len(list))

	require.Equal(t, "c", list[0].Cover, "expected requested order, got %+v", list)
	require.Equal(t, "a", list[1].Cover, "expected requested order, got %+v", list)
	require.Equal(t, "b", list[2].Cover, "expected requested order, got %+v", list)

	empty, err := repo.ListVisibleByIDs(ctx, enum.BannerTypeUser, nil)
	require.NoError(t, err, "list visible by empty ids failed: %v", err)

	require.NotNil(t, empty, "empty id list should return empty non-nil slice, got %+v", empty)
	require.Empty(t, empty, "empty id list should return empty non-nil slice, got %+v", empty)
}

func TestBannerRepositoryUpdateAndDelete(t *testing.T) {
	t.Parallel()

	repo, _, _ := newBannerFixture(t)
	ctx := context.Background()

	banner := &domain.Banner{Cover: "old", Show: true, Type: enum.BannerTypeHome}
	{
		err := repo.Create(ctx, banner)
		require.NoError(t, err, "create banner failed: %v", err)
	}

	cover := "new"
	show := false
	bannerType := enum.BannerTypeTop
	partitionID := uint(0)

	{
		err := repo.Update(ctx, banner.ID, domain.BannerChanges{
			Cover:       &cover,
			Show:        &show,
			Type:        &bannerType,
			PartitionID: &partitionID,
		})
		require.NoError(t, err, "update banner failed: %v", err)
	}

	updated, err := repo.FindByID(ctx, banner.ID)
	require.NoError(t, err, "find updated banner failed: %v", err)

	require.Equal(t, "new", updated.Cover, "unexpected updated banner: %+v", updated)
	require.False(t, updated.Show, "unexpected updated banner: %+v", updated)
	require.Equal(t, enum.BannerTypeTop, updated.Type, "unexpected updated banner: %+v", updated)

	// show=false 后不再出现在可见列表
	visible, err := repo.ListVisible(ctx, 0, nil)
	require.NoError(t, err, "list visible failed: %v", err)

	require.Empty(t, visible, "hidden banner should not be visible, got %+v", visible)

	{
		err := repo.Update(ctx, banner.ID, domain.BannerChanges{})
		require.Error(t, err, "empty changes should be rejected")
	}

	{
		err := repo.Delete(ctx, []uint{banner.ID})
		require.NoError(t, err, "delete banner failed: %v", err)
	}

	deleted, err := repo.FindByID(ctx, banner.ID)
	require.NoError(t, err, "find deleted banner failed: %v", err)

	require.Nil(t, deleted, "banner should be deleted, got %+v", deleted)
}

func TestVideoPartitionQueryExists(t *testing.T) {
	t.Parallel()

	_, partitions, db := newBannerFixture(t)
	ctx := context.Background()

	row := po.VideoPartition{Name: "动画"}
	{
		err := db.Create(&row).Error
		require.NoError(t, err, "seed partition failed: %v", err)
	}

	exists, err := partitions.Exists(ctx, row.ID)
	require.NoError(t, err, "partition exists failed: %v", err)

	require.True(t, exists, "seeded partition should exist")

	exists, err = partitions.Exists(ctx, row.ID+999)
	require.NoError(t, err, "partition exists failed: %v", err)

	require.False(t, exists, "missing partition should not exist")
}
