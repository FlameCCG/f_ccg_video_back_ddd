package comment

import (
	"bytes"
	"context"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/comment"
	domain "f_ccg_video_back_ddd/internal/domain/comment"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type pinTransaction struct {
	tx.Manager
	before func()
}

func (m pinTransaction) Do(ctx context.Context, fn func(context.Context) error) error {
	if m.before != nil {
		m.before()
	}
	return m.Manager.Do(ctx, fn)
}

type pinAuthor struct {
	app.RelatedQuery
	t *testing.T
}

func (p pinAuthor) VideoAuthorRef(ctx context.Context, _ uint) (*app.VideoAuthorRef, error) {
	if gormx.TxFrom(ctx) == nil {
		p.t.Fatal("authorship lookup lost the pin transaction context")
	}
	return &app.VideoAuthorRef{AuthorUserID: 9}, nil
}

func newPinDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "pins.db")), &gorm.Config{
		IgnoreRelationshipsWhenMigrating: true, DisableForeignKeyConstraintWhenMigrating: true,
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.AutoMigrate(&po.Comment{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	rows := []po.Comment{
		{Model: po.Model{ID: 1}, VideoID: 10, UserID: 7, Content: "old", IsPinned: true, PinnedAt: &now},
		{Model: po.Model{ID: 2}, VideoID: 10, UserID: 8, Content: "target", LikeCount: 12, ReplyCount: 3},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestPinRejectsTargetDeletedBeforeTransactionAndKeepsPreviousPin(t *testing.T) {
	db := newPinDatabase(t)
	txm := pinTransaction{Manager: gormx.NewTxManager(db), before: func() {
		if err := db.Delete(&po.Comment{}, 2).Error; err != nil {
			t.Fatal(err)
		}
	}}
	service := app.NewPinService(txm, NewRepository(db), pinAuthor{t: t})
	if err := service.Pin(t.Context(), 9, 2, true); !errors.Is(err, domain.ErrCommentNotFound) {
		t.Fatalf("deleted target must be rejected, got %v", err)
	}
	var old po.Comment
	if err := db.First(&old, 1).Error; err != nil || !old.IsPinned || old.PinnedAt == nil {
		t.Fatalf("previous pin lost: %+v, err=%v", old, err)
	}
}

func TestPinReplacementFailureRollsBackPreviousPin(t *testing.T) {
	db := newPinDatabase(t)
	failure := errors.New("pin write failed")
	if err := db.Callback().Update().Before("gorm:update").Register("test:fail_pin", func(db *gorm.DB) {
		if changes, ok := db.Statement.Dest.(map[string]any); ok && changes["is_pinned"] == true {
			db.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	service := app.NewPinService(gormx.NewTxManager(db), NewRepository(db), pinAuthor{t: t})
	if err := service.Pin(t.Context(), 9, 2, true); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	var rows []po.Comment
	if err := db.Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !rows[0].IsPinned || rows[0].PinnedAt == nil || rows[1].IsPinned || rows[1].LikeCount != 12 || rows[1].ReplyCount != 3 {
		t.Fatalf("replacement was not rolled back or changed unrelated fields: %+v", rows)
	}
}

type disappearingPinRepository struct{ *Repository }

func (r disappearingPinRepository) SetPinned(ctx context.Context, id uint, pinned bool, at *time.Time) error {
	if err := r.DeleteByID(ctx, id); err != nil {
		return err
	}
	return r.Repository.SetPinned(ctx, id, pinned, at)
}

func TestPinZeroRowWriteRejectsMissingTargetAndRollsBack(t *testing.T) {
	db := newPinDatabase(t)
	service := app.NewPinService(gormx.NewTxManager(db), disappearingPinRepository{NewRepository(db)}, pinAuthor{t: t})
	if err := service.Pin(t.Context(), 9, 2, true); !errors.Is(err, domain.ErrCommentNotFound) {
		t.Fatalf("zero row write must fail: %v", err)
	}
	var old, target po.Comment
	if err := db.First(&old, 1).Error; err != nil || !old.IsPinned || old.PinnedAt == nil {
		t.Fatalf("old pin was not rolled back: %+v %v", old, err)
	}
	if err := db.First(&target, 2).Error; err != nil || target.IsPinned {
		t.Fatalf("transaction did not restore the target: %+v %v", target, err)
	}
}

func TestPinReplacementPreservesCountersAndRepeatedUnpinSucceeds(t *testing.T) {
	db := newPinDatabase(t)
	service := app.NewPinService(gormx.NewTxManager(db), NewRepository(db), pinAuthor{t: t})
	for _, pinned := range []bool{true, false, false} {
		if err := service.Pin(t.Context(), 9, 2, pinned); err != nil {
			t.Fatal(err)
		}
		var old, target po.Comment
		if err := db.First(&old, 1).Error; err != nil || old.IsPinned || old.PinnedAt != nil {
			t.Fatalf("old pin was not replaced: %+v %v", old, err)
		}
		if err := db.First(&target, 2).Error; err != nil || target.IsPinned != pinned || (target.PinnedAt != nil) != pinned || target.LikeCount != 12 || target.ReplyCount != 3 || target.Content != "target" {
			t.Fatalf("incorrect pin state or unrelated fields changed: %+v %v", target, err)
		}
	}
}

// SQLite 验证实际回滚；MySQL 方言另行验证目标查询包含行锁。
func TestPinTargetReadUsesMySQLRowLock(t *testing.T) {
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
	if _, err := NewRepository(db).FindPinTargetForUpdate(t.Context(), 7); err != nil {
		t.Fatal(err)
	}
	if sql := trace.String(); !strings.Contains(sql, "FOR UPDATE") || !strings.Contains(sql, "`comments`.`id` = 7") {
		t.Fatalf("pin target query lacks identity or row lock: %s", sql)
	}
}
