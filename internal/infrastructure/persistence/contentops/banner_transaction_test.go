package contentops

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/contentops"
	contentopsdomain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type failingBannerWrite struct {
	*BannerRepository
	failure error
}

func (r failingBannerWrite) Update(ctx context.Context, id uint, changes contentopsdomain.BannerChanges) error {
	if err := r.BannerRepository.Update(ctx, id, changes); err != nil {
		return err
	}
	return r.failure
}

type bannerCommitCache struct {
	app.BannerCache
	invalidations int
	onInvalidate  func()
}

func (c *bannerCommitCache) InvalidateVisible() {
	c.invalidations++
	if c.onInvalidate != nil {
		c.onInvalidate()
	}
}

func TestBannerPlacementRollbackAndCommitKeepUnrelatedFields(t *testing.T) {
	repo, partitions, db := newBannerFixture(t)
	row := po.Banner{Cover: "old", Href: "unchanged", Type: enum.BannerTypeHome, PartitionID: 7}
	seedBanner(t, db, &row, false)
	cache := &bannerCommitCache{}
	failure := errors.New("failed after placement write")
	txm := gormx.NewTxManager(db)
	svc := app.NewBannerService(txm, failingBannerWrite{repo, failure}, cache, partitions)
	nonHome, cover := int(enum.BannerTypeTop), "new"
	params := app.UpdateBannerParams{ID: row.ID, Type: &nonHome, Cover: &cover}
	if err := svc.Update(t.Context(), params); !errors.Is(err, failure) {
		t.Fatalf("placement write error = %v", err)
	}
	var stored po.Banner
	if err := db.First(&stored, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Type != enum.BannerTypeHome || stored.PartitionID != 7 || stored.Cover != "old" || stored.Href != "unchanged" || stored.Show {
		t.Fatalf("failed placement did not roll back all fields: %+v", stored)
	}
	if cache.invalidations != 0 {
		t.Fatal("failed transaction invalidated visible cache")
	}
	cache.onInvalidate = func() {
		if err := db.First(&stored, row.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.Type != enum.BannerTypeTop || stored.PartitionID != 0 || stored.Cover != cover || stored.Href != "unchanged" || stored.Show {
			t.Fatalf("cache invalidated before placement committed or unrelated fields overwritten: %+v", stored)
		}
	}
	svc = app.NewBannerService(txm, repo, cache, partitions)
	if err := svc.Update(t.Context(), params); err != nil {
		t.Fatal(err)
	}
	if cache.invalidations != 1 {
		t.Fatalf("successful placement invalidations = %d", cache.invalidations)
	}
	missing := row.ID + 999
	if err := txm.Do(t.Context(), func(ctx context.Context) error {
		banner, err := repo.FindByIDForUpdate(ctx, missing)
		if banner != nil {
			t.Fatal("missing placement read returned a banner")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

// SQLite 的事务回滚测试不能证明 MySQL 行锁；另用 MySQL 方言检查实际锁定查询。
func TestBannerPlacementReadUsesMySQLRowLock(t *testing.T) {
	var trace bytes.Buffer
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN: "unused:unused@tcp(127.0.0.1:1)/dryrun", SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun: true, DisableAutomaticPing: true,
		Logger: logger.New(log.New(&trace, "", 0), logger.Config{LogLevel: logger.Info}),
	})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := NewBannerRepository(db).FindByIDForUpdate(t.Context(), 7); err != nil {
		t.Fatal(err)
	}
	sql := trace.String()
	if !strings.Contains(sql, "FOR UPDATE") || !strings.Contains(sql, "`banners`.`id` = 7") {
		t.Fatalf("placement query lacks identity or row lock: %s", sql)
	}
}
