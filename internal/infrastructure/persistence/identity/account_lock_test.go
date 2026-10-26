package identity_test

import (
	"bytes"
	"log"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 使用 MySQL 方言生成 SQL，不连接数据库；SQLite 不支持验证 FOR UPDATE。
func TestLockAccountsUsesOrderedMySQLRowLocks(t *testing.T) {
	t.Parallel()

	var trace bytes.Buffer

	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN: "unused:unused@tcp(127.0.0.1:1)/dryrun", SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun: true, DisableAutomaticPing: true,
		Logger: logger.New(log.New(&trace, "", 0), logger.Config{LogLevel: logger.Info}),
	})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)

	t.Cleanup(func() { _ = sqlDB.Close() })

	ids := []uint{9, 0, 2, 9}
	{
		err := infra.NewUserRepository(db, nil).LockAccounts(t.Context(), ids)
		require.NoError(t, err)
	}

	require.True(t, slices.Equal(ids, []uint{9, 0, 2, 9}), "caller-owned IDs mutated")

	sql := trace.String()

	first, second := strings.Index(sql, "id = 2"), strings.Index(sql, "id = 9")
	require.Equal(t, 2, strings.Count(sql, "FOR UPDATE"), "expected two ascending, deduplicated row locks: %s", sql)
	require.GreaterOrEqual(t, first, 0, "expected two ascending, deduplicated row locks: %s", sql)
	require.GreaterOrEqual(t, second, first, "expected two ascending, deduplicated row locks: %s", sql)
	require.NotContains(t, sql, "id = 0", "expected two ascending, deduplicated row locks: %s", sql)
}
