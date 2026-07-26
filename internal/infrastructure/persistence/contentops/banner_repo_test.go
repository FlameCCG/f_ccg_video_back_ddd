package contentops

import (
	"context"
	"path/filepath"
	"testing"

	domain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

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
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	if err := db.AutoMigrate(&po.Banner{}, &po.VideoPartition{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}

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
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("seed banner failed: %v", err)
	}

	if err := db.Model(&po.Banner{}).Where("id = ?", row.ID).Update("show", show).Error; err != nil {
		t.Fatalf("seed banner show failed: %v", err)
	}

	row.Show = show
}

func TestBannerRepositoryCreateAndRead(t *testing.T) {
	repo, _, _ := newBannerFixture(t)
	ctx := context.Background()

	banner := &domain.Banner{
		Cover:       "https://example.com/a.png",
		Href:        "https://example.com",
		Show:        true,
		Type:        enum.BannerTypeHome,
		PartitionID: 0,
	}
	if err := repo.Create(ctx, banner); err != nil {
		t.Fatalf("create banner failed: %v", err)
	}

	if banner.ID == 0 {
		t.Fatal("create should backfill auto increment id")
	}

	if banner.CreatedAt.IsZero() {
		t.Fatal("create should backfill created_at")
	}

	found, err := repo.FindByID(ctx, banner.ID)
	if err != nil {
		t.Fatalf("find banner failed: %v", err)
	}

	if found == nil || found.Cover != banner.Cover || found.Type != enum.BannerTypeHome {
		t.Fatalf("unexpected banner: %+v", found)
	}

	missing, err := repo.FindByID(ctx, banner.ID+999)
	if err != nil {
		t.Fatalf("find missing banner should not error: %v", err)
	}

	if missing != nil {
		t.Fatalf("missing banner should be nil, got %+v", missing)
	}
}

func TestBannerRepositoryListVisibleFiltersAndOrder(t *testing.T) {
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
	if err != nil {
		t.Fatalf("list visible failed: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 visible home banners, got %d (%+v)", len(list), list)
	}

	// id DESC 排序
	if list[0].Cover != "home-2" || list[1].Cover != "home-1" {
		t.Fatalf("expected id desc order, got %+v", list)
	}

	// 不限类型、不限分区时返回全部可见项
	all, err := repo.ListVisible(ctx, 0, nil)
	if err != nil {
		t.Fatalf("list visible all failed: %v", err)
	}

	if len(all) != 4 {
		t.Fatalf("expected 4 visible banners, got %d", len(all))
	}

	// 无匹配数据：GORM 会把目标切片初始化为非 nil 空切片，
	// 因此响应体序列化为 []（与原实现一致，不是 null）。
	empty, err := repo.ListVisible(ctx, enum.BannerTypeUser, nil)
	if err != nil {
		t.Fatalf("list visible empty failed: %v", err)
	}

	if empty == nil || len(empty) != 0 {
		t.Fatalf("expected empty non-nil slice, got %+v", empty)
	}
}

func TestBannerRepositoryListAdminPaging(t *testing.T) {
	repo, _, db := newBannerFixture(t)
	ctx := context.Background()

	for i := range 5 {
		row := po.Banner{Cover: "cover", Type: enum.BannerTypeHome, PartitionID: uint(i % 2)}
		seedBanner(t, db, &row, true)
	}

	query := domain.BannerAdminQuery{Page: 1, PageSize: 2, Type: enum.BannerTypeHome}
	query.Normalize()

	list, total, err := repo.ListAdmin(ctx, query)
	if err != nil {
		t.Fatalf("list admin failed: %v", err)
	}

	if total != 5 {
		t.Fatalf("unexpected total: %d", total)
	}

	if len(list) != 2 {
		t.Fatalf("unexpected page size: %d", len(list))
	}

	partitionID := uint(1)
	filtered := domain.BannerAdminQuery{PartitionID: &partitionID}
	filtered.Normalize()

	list, total, err = repo.ListAdmin(ctx, filtered)
	if err != nil {
		t.Fatalf("list admin by partition failed: %v", err)
	}

	if total != 2 || len(list) != 2 {
		t.Fatalf("unexpected filtered result: total=%d len=%d", total, len(list))
	}

	missingPartition := uint(99)
	none := domain.BannerAdminQuery{PartitionID: &missingPartition}
	none.Normalize()

	list, total, err = repo.ListAdmin(ctx, none)
	if err != nil {
		t.Fatalf("list admin empty failed: %v", err)
	}

	if total != 0 || list == nil || len(list) != 0 {
		t.Fatalf("empty admin list should be an empty non-nil slice, got total=%d list=%+v", total, list)
	}
}

func TestBannerRepositoryListVisibleByIDsKeepsRequestedOrder(t *testing.T) {
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
	if err != nil {
		t.Fatalf("list visible by ids failed: %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 banners (missing id skipped), got %d", len(list))
	}

	if list[0].Cover != "c" || list[1].Cover != "a" || list[2].Cover != "b" {
		t.Fatalf("expected requested order, got %+v", list)
	}

	empty, err := repo.ListVisibleByIDs(ctx, enum.BannerTypeUser, nil)
	if err != nil {
		t.Fatalf("list visible by empty ids failed: %v", err)
	}

	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty id list should return empty non-nil slice, got %+v", empty)
	}
}

func TestBannerRepositoryUpdateAndDelete(t *testing.T) {
	repo, _, _ := newBannerFixture(t)
	ctx := context.Background()

	banner := &domain.Banner{Cover: "old", Show: true, Type: enum.BannerTypeHome}
	if err := repo.Create(ctx, banner); err != nil {
		t.Fatalf("create banner failed: %v", err)
	}

	cover := "new"
	show := false
	bannerType := enum.BannerTypeTop
	partitionID := uint(0)

	if err := repo.Update(ctx, banner.ID, domain.BannerChanges{
		Cover:       &cover,
		Show:        &show,
		Type:        &bannerType,
		PartitionID: &partitionID,
	}); err != nil {
		t.Fatalf("update banner failed: %v", err)
	}

	updated, err := repo.FindByID(ctx, banner.ID)
	if err != nil {
		t.Fatalf("find updated banner failed: %v", err)
	}

	if updated.Cover != "new" || updated.Show || updated.Type != enum.BannerTypeTop {
		t.Fatalf("unexpected updated banner: %+v", updated)
	}

	// show=false 后不再出现在可见列表
	visible, err := repo.ListVisible(ctx, 0, nil)
	if err != nil {
		t.Fatalf("list visible failed: %v", err)
	}

	if len(visible) != 0 {
		t.Fatalf("hidden banner should not be visible, got %+v", visible)
	}

	if err := repo.Update(ctx, banner.ID, domain.BannerChanges{}); err == nil {
		t.Fatal("empty changes should be rejected")
	}

	if err := repo.Delete(ctx, []uint{banner.ID}); err != nil {
		t.Fatalf("delete banner failed: %v", err)
	}

	deleted, err := repo.FindByID(ctx, banner.ID)
	if err != nil {
		t.Fatalf("find deleted banner failed: %v", err)
	}

	if deleted != nil {
		t.Fatalf("banner should be deleted, got %+v", deleted)
	}
}

func TestVideoPartitionQueryExists(t *testing.T) {
	_, partitions, db := newBannerFixture(t)
	ctx := context.Background()

	row := po.VideoPartition{Name: "动画"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed partition failed: %v", err)
	}

	exists, err := partitions.Exists(ctx, row.ID)
	if err != nil {
		t.Fatalf("partition exists failed: %v", err)
	}

	if !exists {
		t.Fatal("seeded partition should exist")
	}

	exists, err = partitions.Exists(ctx, row.ID+999)
	if err != nil {
		t.Fatalf("partition exists failed: %v", err)
	}

	if exists {
		t.Fatal("missing partition should not exist")
	}
}
