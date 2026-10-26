package identity_test

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"path/filepath"
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"
	domain "f_ccg_video_back_ddd/internal/domain/identity"
	identityrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type banStatusCache struct {
	writes map[uint]enum.UserStatus
}

func newBanStatusCache() *banStatusCache {
	return &banStatusCache{writes: make(map[uint]enum.UserStatus)}
}

func (*banStatusCache) Load(context.Context, uint) (enum.UserStatus, bool) {
	return 0, false
}

func (*banStatusCache) Generation(context.Context, uint) int64 {
	return 0
}

func (*banStatusCache) StoreIfGenerationMatch(context.Context, uint, enum.UserStatus, int64) {}

func (c *banStatusCache) Write(userID uint, status enum.UserStatus) {
	c.writes[userID] = status
}

func (*banStatusCache) Invalidate(uint) {}

var _ domain.UserStatusCache = (*banStatusCache)(nil)

type banFixture struct {
	service *app.BanService
	db      *gorm.DB
	cache   *banStatusCache
}

func newBanFixture(t *testing.T) banFixture {
	t.Helper()

	db, err := gorm.Open(
		sqlite.Open(filepath.Join(t.TempDir(), "ban-service.db")),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	statements := []string{
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			created_at DATETIME,
			updated_at DATETIME,
			username TEXT NOT NULL,
			avatar TEXT,
			status INTEGER NOT NULL
		)`,
		`CREATE TABLE user_ban_records (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at DATETIME,
			updated_at DATETIME,
			user_id INTEGER NOT NULL,
			status INTEGER NOT NULL,
			reason TEXT,
			days INTEGER NOT NULL DEFAULT 0,
			start_at DATETIME,
			end_at DATETIME,
			operator_id INTEGER NOT NULL DEFAULT 0,
			is_unbanned NUMERIC NOT NULL DEFAULT 0
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create ban test schema: %v", err)
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite connection: %v", err)
	}

	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close sqlite connection: %v", err)
		}
	})

	users := identityrepo.NewUserRepository(db, nil)
	records := identityrepo.NewBanRecordRepository(db)
	cache := newBanStatusCache()
	statuses := app.NewUserStatusService(cache, users)

	return banFixture{
		service: app.NewBanService(gormx.NewTxManager(db), users, records, statuses, nil),
		db:      db,
		cache:   cache,
	}
}

func TestBanServiceApplyUnbanMarksOriginalRecordWithoutCreatingActiveRecord(t *testing.T) {
	t.Parallel()

	fixture := newBanFixture(t)
	record := seedBannedUser(t, fixture.db, time.Now().Add(24*time.Hour))

	result, err := fixture.service.Apply(context.Background(), domain.BanAction{
		UserID: 1,
		Status: enum.UserStatusActive,
		Reason: "管理员解封",
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if result.ID != record.ID || result.Status != enum.UserStatusActive || !result.IsUnbanned {
		t.Fatalf("Apply() result = %#v, want original record id %d and active event", result, record.ID)
	}

	assertUserAndRecordUnbanned(t, fixture.db, record.ID)
	assertBanRecordCount(t, fixture.db, 1)

	latest, err := identityrepo.NewBanRecordRepository(fixture.db).
		FindLatestByUserID(context.Background(), 1)
	if err != nil {
		t.Fatalf("FindLatestByUserID() error = %v", err)
	}

	if latest != nil {
		t.Fatalf("FindLatestByUserID() = %+v, want nil after release", latest)
	}

	if fixture.cache.writes[1] != enum.UserStatusActive {
		t.Fatalf("status cache writes = %v, want user 1 active", fixture.cache.writes)
	}
}

func TestBanServiceReleaseExpiredMarksOriginalRecordOnce(t *testing.T) {
	t.Parallel()

	fixture := newBanFixture(t)
	record := seedBannedUser(t, fixture.db, time.Now().Add(-time.Hour))

	unbanned, err := fixture.service.ReleaseExpired(context.Background(), 1, time.Now())
	if err != nil {
		t.Fatalf("ReleaseExpired() error = %v", err)
	}

	if !unbanned {
		t.Fatal("ReleaseExpired() unbanned = false, want true")
	}

	assertUserAndRecordUnbanned(t, fixture.db, record.ID)
	assertBanRecordCount(t, fixture.db, 1)

	unbanned, err = fixture.service.ReleaseExpired(context.Background(), 1, time.Now())
	if err != nil {
		t.Fatalf("second ReleaseExpired() error = %v", err)
	}

	if unbanned {
		t.Fatal("second ReleaseExpired() unbanned = true, want false")
	}
}

func TestBanServiceReleaseExpiredDoesNotClaimReleaseAfterStatusChanged(t *testing.T) {
	t.Parallel()

	fixture := newBanFixture(t)

	record := seedBannedUser(t, fixture.db, time.Now().Add(-time.Hour))
	if err := fixture.db.Model(&po.User{}).
		Where("id = ?", 1).
		Update("status", enum.UserStatusActive).Error; err != nil {
		t.Fatalf("change user status: %v", err)
	}

	unbanned, err := fixture.service.ReleaseExpired(context.Background(), 1, time.Now())
	if err != nil {
		t.Fatalf("ReleaseExpired() error = %v", err)
	}

	if unbanned {
		t.Fatal("ReleaseExpired() unbanned = true, want false")
	}

	var stored po.UserBanRecord
	if err := fixture.db.First(&stored, record.ID).Error; err != nil {
		t.Fatalf("load ban record: %v", err)
	}

	if stored.IsUnbanned {
		t.Fatal("record is_unbanned = true, want false when conditional activation loses")
	}

	if len(fixture.cache.writes) != 0 {
		t.Fatalf("unexpected status cache writes: %v", fixture.cache.writes)
	}
}

func TestBanRecordQueryReportsHistoricalActiveRelease(t *testing.T) {
	t.Parallel()

	fixture := newBanFixture(t)
	if err := fixture.db.Exec(
		"INSERT INTO users (id, username, status) VALUES (?, ?, ?)",
		1,
		"released-user",
		enum.UserStatusActive,
	).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	createdAt := time.Now().Add(-2 * time.Hour)

	ban := po.UserBanRecord{
		Model:      po.Model{CreatedAt: createdAt},
		UserID:     1,
		Status:     enum.UserStatusBanned,
		Reason:     "历史封禁",
		Days:       1,
		StartAt:    createdAt,
		IsUnbanned: false,
	}
	if err := fixture.db.Create(&ban).Error; err != nil {
		t.Fatalf("create historical ban: %v", err)
	}

	legacyActive := po.UserBanRecord{
		Model:      po.Model{CreatedAt: createdAt.Add(time.Hour)},
		UserID:     1,
		Status:     enum.UserStatusActive,
		Reason:     "自动解封",
		StartAt:    createdAt.Add(time.Hour),
		IsUnbanned: false,
	}
	if err := fixture.db.Create(&legacyActive).Error; err != nil {
		t.Fatalf("create historical active record: %v", err)
	}

	items, total, err := identityrepo.NewBanRecordQuery(fixture.db).AdminList(
		context.Background(),
		app.AdminBanRecordListParams{UserID: 1},
	)
	if err != nil {
		t.Fatalf("AdminList() error = %v", err)
	}

	if total != 1 || len(items) != 1 {
		t.Fatalf("AdminList() total/items = %d/%d, want 1/1", total, len(items))
	}

	if items[0].ID != ban.ID || !items[0].IsUnbanned {
		t.Fatalf("AdminList() item = %+v, want historical ban marked unbanned", items[0])
	}
}

func seedBannedUser(t *testing.T, db *gorm.DB, endAt time.Time) po.UserBanRecord {
	t.Helper()

	if err := db.Exec(
		"INSERT INTO users (id, username, status) VALUES (?, ?, ?)",
		1,
		"banned-user",
		enum.UserStatusBanned,
	).Error; err != nil {
		t.Fatalf("seed banned user: %v", err)
	}

	record := po.UserBanRecord{
		UserID:     1,
		Status:     enum.UserStatusBanned,
		Reason:     "测试封禁",
		Days:       1,
		StartAt:    endAt.Add(-24 * time.Hour),
		EndAt:      &endAt,
		IsUnbanned: false,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed ban record: %v", err)
	}

	return record
}

func assertUserAndRecordUnbanned(t *testing.T, db *gorm.DB, recordID uint) {
	t.Helper()

	var userStatus enum.UserStatus
	if err := db.Table("users").Select("status").Where("id = ?", 1).Scan(&userStatus).Error; err != nil {
		t.Fatalf("query user status: %v", err)
	}

	if userStatus != enum.UserStatusActive {
		t.Fatalf("user status = %d, want active", userStatus)
	}

	var record po.UserBanRecord
	if err := db.First(&record, recordID).Error; err != nil {
		t.Fatalf("query ban record: %v", err)
	}

	if !record.IsUnbanned {
		t.Fatal("record is_unbanned = false, want true")
	}
}

func assertBanRecordCount(t *testing.T, db *gorm.DB, expected int64) {
	t.Helper()

	var count int64
	if err := db.Model(&po.UserBanRecord{}).Where("user_id = ?", 1).Count(&count).Error; err != nil {
		t.Fatalf("count ban records: %v", err)
	}

	if count != expected {
		t.Fatalf("ban record count = %d, want %d", count, expected)
	}
}
