package video

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"path/filepath"
	"testing"
	"time"
)

func TestReportHandlingPersistsOnlyPendingRowsAndAssociatedFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "handling.db")), &gorm.Config{IgnoreRelationshipsWhenMigrating: true, DisableForeignKeyConstraintWhenMigrating: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&po.VideoReport{}, &po.DanmuReport{}, &po.VideoPartition{}); err != nil {
		t.Fatal(err)
	}
	oldAt := time.Now().Add(-time.Hour).Truncate(time.Millisecond)
	rows := []po.VideoReport{
		{Model: po.Model{ID: 1}, Reason: "keep", Status: enum.ReportStatusPending},
		{Model: po.Model{ID: 2}, Reason: "handled", Status: enum.ReportStatusResolved, HandleNote: "old", HandledBy: 9, HandledAt: &oldAt},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	danmu := po.DanmuReport{Model: po.Model{ID: 3}, Status: enum.ReportStatusPending, Reason: "danmu"}
	if err := db.Create(&danmu).Error; err != nil {
		t.Fatal(err)
	}
	at := time.Now().Truncate(time.Millisecond)
	plan, err := (domain.VideoReport{}).PlanHandling(enum.ReportStatusRejected, " reject ", 7, at)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewReportRepository(db)
	ctx := context.Background()
	if affected, err := repo.HandleVideoReports(ctx, []uint{1, 2, 999}, plan); err != nil || affected != 1 {
		t.Fatal(affected, err)
	}
	var pending, handled po.VideoReport
	if err := db.First(&pending, 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&handled, 2).Error; err != nil {
		t.Fatal(err)
	}
	if pending.Status != enum.ReportStatusRejected || pending.HandleNote != "reject" || pending.HandledBy != 7 || pending.HandledAt == nil || !pending.HandledAt.Equal(at) || pending.Reason != "keep" {
		t.Fatal(pending)
	}
	if handled.HandleNote != "old" || handled.HandledBy != 9 || !handled.HandledAt.Equal(oldAt) || handled.Status != enum.ReportStatusResolved {
		t.Fatal("already handled report changed", handled)
	}
	if affected, err := repo.HandleDanmuReports(ctx, []uint{3}, plan); err != nil || affected != 1 {
		t.Fatal(affected, err)
	}
	if affected, err := repo.HandleVideoReports(ctx, []uint{1}, domain.ReportHandling{}); affected != 0 || !errors.Is(err, domain.ErrReportStatusInvalid) {
		t.Fatal(affected, err)
	}
	input := &domain.Partition{Name: "new", IsActive: true, IsSubmittable: true}
	created, err := NewPartitionRepository(db).Create(ctx, input)
	if err != nil || created.ID == 0 || input.ID != 0 || created.Name != input.Name {
		t.Fatal(created, input, err)
	}
}
