package dynamic_test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/dynamic"
	domain "f_ccg_video_back_ddd/internal/domain/dynamic"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/dynamic"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	videoinfra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"

	"go.uber.org/zap"
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

func newPinFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	now := time.Now().Truncate(time.Second)
	for _, model := range []any{&po.Dynamic{}, &po.Video{}} {
		if err := f.db.Model(model).Where("id = 1").Updates(map[string]any{"is_pinned": true, "pinned_at": &now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	var target po.Video
	if err := f.db.First(&target, 1).Error; err != nil {
		t.Fatal(err)
	}
	target.Model = po.Model{ID: 2}
	target.Title, target.IsPinned, target.PinnedAt = "target", false, nil
	if err := f.db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	return f
}

func pinService(f *fixture, manager tx.Manager) *app.Service {
	return app.NewService(manager, f.dynamics, f.users, f.videos, videoinfra.NewPinRepository(f.db), nil, zap.NewNop())
}

func assertPreviousPins(t *testing.T, f *fixture, pinned bool) {
	t.Helper()
	var dynamic po.Dynamic
	var video po.Video
	if err := f.db.First(&dynamic, 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.First(&video, 1).Error; err != nil {
		t.Fatal(err)
	}
	if dynamic.IsPinned != pinned || (dynamic.PinnedAt != nil) != pinned || video.IsPinned != pinned || (video.PinnedAt != nil) != pinned {
		t.Fatalf("previous pins: dynamic=%+v video=%+v; want pinned=%v", dynamic, video, pinned)
	}
}

func TestDynamicPinRechecksTargetInsideTransaction(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*fixture) error
		want   error
	}{
		{"deleted", func(f *fixture) error { return f.db.Delete(&po.Dynamic{}, 2).Error }, domain.ErrDynamicNotFound},
		{"changed owner", func(f *fixture) error { return f.db.Model(&po.Dynamic{}).Where("id = 2").Update("user_id", 3).Error }, domain.ErrPinForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPinFixture(t)
			manager := pinTransaction{Manager: gormx.NewTxManager(f.db), before: func() {
				if err := tc.mutate(f); err != nil {
					t.Fatal(err)
				}
			}}
			if err := pinService(f, manager).Pin(t.Context(), 1, 2, true); !errors.Is(err, tc.want) {
				t.Fatalf("Pin=%v; want %v", err, tc.want)
			}
			assertPreviousPins(t, f, true)
		})
	}
}

func TestVideoPinRechecksTargetInsideTransaction(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*fixture) error
	}{
		{"deleted", func(f *fixture) error { return f.db.Delete(&po.Video{}, 2).Error }},
		{"changed owner", func(f *fixture) error {
			return f.db.Model(&po.Video{}).Where("id = 2").Update("author_username", "carol").Error
		}},
		{"unpublished", func(f *fixture) error {
			return f.db.Model(&po.Video{}).Where("id = 2").Update("status", enum.VideoStatusPrivate).Error
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPinFixture(t)
			manager := pinTransaction{Manager: gormx.NewTxManager(f.db), before: func() {
				if err := tc.mutate(f); err != nil {
					t.Fatal(err)
				}
			}}
			if err := pinService(f, manager).PinVideo(t.Context(), 1, 2, true); !errors.Is(err, domain.ErrVideoPinForbidden) {
				t.Fatalf("PinVideo=%v; want %v", err, domain.ErrVideoPinForbidden)
			}
			assertPreviousPins(t, f, true)
		})
	}
}

func TestPinReplacementWriteFailureRollsBackBothContexts(t *testing.T) {
	for _, video := range []bool{false, true} {
		name := "dynamic"
		if video {
			name = "video"
		}
		t.Run(name, func(t *testing.T) {
			f := newPinFixture(t)
			failure := errors.New("pin target write failed")
			if err := f.db.Callback().Update().Before("gorm:update").Register("test:reject_pin", func(db *gorm.DB) {
				if changes, ok := db.Statement.Dest.(map[string]any); ok && changes["is_pinned"] == true {
					db.AddError(failure)
				}
			}); err != nil {
				t.Fatal(err)
			}
			service := pinService(f, gormx.NewTxManager(f.db))
			var err error
			if video {
				err = service.PinVideo(t.Context(), 1, 2, true)
			} else {
				err = service.Pin(t.Context(), 1, 2, true)
			}
			if !errors.Is(err, failure) {
				t.Fatalf("write failure lost: %v", err)
			}
			assertPreviousPins(t, f, true)
		})
	}
}

type disappearingDynamicPin struct {
	domain.DynamicRepository
	db *gorm.DB
}

func (r disappearingDynamicPin) SetPinned(ctx context.Context, id uint, at *time.Time) error {
	if err := gormx.DB(ctx, r.db).Delete(&po.Dynamic{}, id).Error; err != nil {
		return err
	}
	return r.DynamicRepository.SetPinned(ctx, id, at)
}

type disappearingVideoPin struct {
	app.VideoPinWriter
	db *gorm.DB
}

func (r disappearingVideoPin) SetPinned(ctx context.Context, id uint, username string, at *time.Time) (bool, error) {
	if err := gormx.DB(ctx, r.db).Delete(&po.Video{}, id).Error; err != nil {
		return false, err
	}
	return r.VideoPinWriter.SetPinned(ctx, id, username, at)
}

func TestPinZeroRowWriteRollsBackPreviousPins(t *testing.T) {
	for _, video := range []bool{false, true} {
		name := "dynamic"
		if video {
			name = "video"
		}
		t.Run(name, func(t *testing.T) {
			f := newPinFixture(t)
			var dynamics domain.DynamicRepository = f.dynamics
			var videos app.VideoPinWriter = videoinfra.NewPinRepository(f.db)
			want := domain.ErrDynamicNotFound
			if video {
				videos = disappearingVideoPin{VideoPinWriter: videos, db: f.db}
				want = domain.ErrVideoPinForbidden
			} else {
				dynamics = disappearingDynamicPin{DynamicRepository: dynamics, db: f.db}
			}
			service := app.NewService(gormx.NewTxManager(f.db), dynamics, f.users, f.videos, videos, nil, zap.NewNop())
			var err error
			if video {
				err = service.PinVideo(t.Context(), 1, 2, true)
			} else {
				err = service.Pin(t.Context(), 1, 2, true)
			}
			if !errors.Is(err, want) {
				t.Fatalf("zero row write=%v; want %v", err, want)
			}
			assertPreviousPins(t, f, true)
			model := any(&po.Dynamic{})
			if video {
				model = &po.Video{}
			}
			var count int64
			if err := f.db.Model(model).Where("id = 2").Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("target deletion was not rolled back: count=%d err=%v", count, err)
			}
		})
	}
}

func TestPinReplacementPreservesOtherFieldsAndRepeatedUnpin(t *testing.T) {
	for _, video := range []bool{false, true} {
		name := "dynamic"
		if video {
			name = "video"
		}
		t.Run(name, func(t *testing.T) {
			f := newPinFixture(t)
			service := pinService(f, gormx.NewTxManager(f.db))
			for _, pinned := range []bool{true, false, false} {
				var err error
				if video {
					err = service.PinVideo(t.Context(), 1, 2, pinned)
				} else {
					err = service.Pin(t.Context(), 1, 2, pinned)
				}
				if err != nil {
					t.Fatal(err)
				}
				assertPreviousPins(t, f, false)
				if video {
					var target po.Video
					if err := f.db.First(&target, 2).Error; err != nil {
						t.Fatal(err)
					}
					if target.IsPinned != pinned || (target.PinnedAt != nil) != pinned || target.Title != "target" || target.Likes != 3 || target.Views != 10 || target.Duration != 60 || target.AuthorUsername != "alice" || target.Status != enum.VideoStatusPublished {
						t.Fatalf("invalid video pin or unrelated field changed: %+v", target)
					}
				} else {
					var target po.Dynamic
					if err := f.db.First(&target, 2).Error; err != nil {
						t.Fatal(err)
					}
					if target.IsPinned != pinned || (target.PinnedAt != nil) != pinned || target.Content != "second" || target.LikeCount != 0 || target.UserID != 1 {
						t.Fatalf("invalid dynamic pin or unrelated field changed: %+v", target)
					}
				}
			}
		})
	}
}

func TestPinTargetReadsUseMySQLRowLocks(t *testing.T) {
	var trace bytes.Buffer
	db, err := gorm.Open(mysql.New(mysql.Config{DSN: "unused:unused@tcp(127.0.0.1:1)/dryrun", SkipInitializeWithVersion: true}), &gorm.Config{
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
	ctx := gormx.WithTx(t.Context(), db)
	if _, err := infra.NewDynamicRepository(db).FindByIDForUpdate(ctx, 7); err != nil {
		t.Fatal(err)
	}
	if sql := trace.String(); !strings.Contains(sql, "FOR UPDATE") || !strings.Contains(sql, "id = 7") {
		t.Fatalf("dynamic row lock missing: %s", sql)
	}
	trace.Reset()
	if _, err := infra.NewVideoOwnershipQuery(db).ExistsPublishedByAuthorForUpdate(ctx, 8, "alice"); err != nil {
		t.Fatal(err)
	}
	if sql := trace.String(); !strings.Contains(sql, "FOR UPDATE") || !strings.Contains(sql, "id = 8") || !strings.Contains(sql, "author_username = 'alice'") || !strings.Contains(sql, "status = 1") {
		t.Fatalf("video row lock or eligibility missing: %s", sql)
	}
}

func TestPinZeroAffectedExistingTargetIsIdempotent(t *testing.T) {
	f := newPinFixture(t)
	if err := f.db.Callback().Update().After("gorm:update").Register("test:mysql_zero_changed_rows", func(db *gorm.DB) {
		if db.Error == nil {
			db.RowsAffected = 0
		}
	}); err != nil {
		t.Fatal(err)
	}
	service := pinService(f, gormx.NewTxManager(f.db))
	for i := 0; i < 2; i++ {
		if err := service.Pin(t.Context(), 1, 2, false); err != nil {
			t.Fatal(err)
		}
		if err := service.PinVideo(t.Context(), 1, 2, false); err != nil {
			t.Fatal(err)
		}
	}
	assertPreviousPins(t, f, true)
	if err := f.dynamics.ClearPinned(t.Context(), 9999); !errors.Is(err, domain.ErrDynamicNotFound) {
		t.Fatalf("missing dynamic: %v", err)
	}
	if exists, err := videoinfra.NewPinRepository(f.db).ClearPinned(t.Context(), 9999, "alice"); err != nil || exists {
		t.Fatalf("missing video: exists=%v err=%v", exists, err)
	}
}
