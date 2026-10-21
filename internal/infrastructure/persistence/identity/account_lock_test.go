package identity_test

import (
	"bytes"
	"log"
	"slices"
	"strings"
	"testing"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 使用 MySQL 方言生成 SQL，不连接数据库；SQLite 不支持验证 FOR UPDATE。
func TestLockAccountsUsesOrderedMySQLRowLocks(t *testing.T) {
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
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	ids := []uint{9, 0, 2, 9}
	if err := infra.NewUserRepository(db, nil).LockAccounts(t.Context(), ids); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []uint{9, 0, 2, 9}) {
		t.Fatal("caller-owned IDs mutated")
	}
	sql := trace.String()
	first, second := strings.Index(sql, "id = 2"), strings.Index(sql, "id = 9")
	if strings.Count(sql, "FOR UPDATE") != 2 || first < 0 || second < first || strings.Contains(sql, "id = 0") {
		t.Fatalf("expected two ascending, deduplicated row locks: %s", sql)
	}
}
