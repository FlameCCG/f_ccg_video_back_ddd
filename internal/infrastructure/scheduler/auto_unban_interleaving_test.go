package scheduler

import (
	"context"
	"errors"
	identityapplication "f_ccg_video_back_ddd/internal/application/identity"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"go.uber.org/zap"
	"testing"
	"time"
)

// Deterministic interleaving: an administrator commits a new ban after the cron query.
type reviewInterleavedBanRepo struct {
	*BanMaintenanceRepository
	afterQuery func()
}

func (r *reviewInterleavedBanRepo) ListExpiredBans(ctx context.Context, now time.Time) ([]identitydomain.ExpiredBan, error) {
	expired, err := r.BanMaintenanceRepository.ListExpiredBans(ctx, now)
	if err == nil && r.afterQuery != nil {
		r.afterQuery()
	}
	return expired, err
}
func TestAutoUnbanDoesNotOverrideNewBan(t *testing.T) {
	for _, permanent := range []bool{false, true} {
		name := "renewed-timed-ban"
		if permanent {
			name = "new-permanent-ban"
		}
		t.Run(name, func(t *testing.T) {
			db := openAutoUnbanTestDB(t)
			seedBanUser(t, db, 1, enum.UserStatusBanned, ptrTime(time.Now().Add(-time.Hour)))
			expected := enum.UserStatus(enum.UserStatusBanned)
			if permanent {
				expected = enum.UserStatusPermanentlyBanned
			}
			repo := &reviewInterleavedBanRepo{BanMaintenanceRepository: NewBanMaintenanceRepository(db)}
			repo.afterQuery = func() {
				end := ptrTime(time.Now().Add(24 * time.Hour))
				if permanent {
					end = nil
				}
				record := po.UserBanRecord{UserID: 1, Status: expected, Reason: "new admin ban", StartAt: time.Now(), EndAt: end}
				if err := db.Create(&record).Error; err != nil {
					t.Fatal(err)
				}
				if err := db.Model(&po.User{}).Where("id = ?", 1).Update("status", expected).Error; err != nil {
					t.Fatal(err)
				}
			}
			cache := newStatusWriteRecorder(db)
			service := identityapplication.NewAutoUnbanService(gormx.NewTxManager(db), repo, cache, zap.NewNop())
			service.Run(context.Background())
			if got := userStatus(t, db, 1); got != expected {
				t.Errorf("new admin ban overwritten: database status=%v expected=%v", got, expected)
			}
			if got, written := cache.writes[1]; written && got != expected {
				t.Errorf("new admin ban masked in cache: cache status=%v expected=%v", got, expected)
			}
		})
	}
}

type failingUnbanMark struct {
	identitydomain.BanMaintenanceRepository
}

func (r failingUnbanMark) MarkBanRecordsUnbanned(context.Context, []uint) error {
	return errors.New("mark failed")
}
func TestAutoUnbanMarkFailureRollsBackStatusAndSkipsCache(t *testing.T) {
	db := openAutoUnbanTestDB(t)
	seedBanUser(t, db, 1, enum.UserStatusBanned, ptrTime(time.Now().Add(-time.Hour)))
	cache := newStatusWriteRecorder(db)
	repo := failingUnbanMark{NewBanMaintenanceRepository(db)}
	identityapplication.NewAutoUnbanService(gormx.NewTxManager(db), repo, cache, zap.NewNop()).Run(context.Background())
	if got := userStatus(t, db, 1); got != enum.UserStatusBanned {
		t.Fatalf("transaction leaked status: %v", got)
	}
	var record po.UserBanRecord
	if err := db.First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.IsUnbanned || len(cache.writes) != 0 {
		t.Fatalf("uncommitted release published: record=%+v cache=%v", record, cache.writes)
	}
}
