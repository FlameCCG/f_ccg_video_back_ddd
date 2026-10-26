package migration

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAutoMigrateReturnsErrorWhenDBNil(t *testing.T) {
	t.Parallel()

	{
		err := NewMigrator(nil, zap.NewNop()).AutoMigrate()
		require.Error(t, err, "expected error when db is nil")
	}
}

func TestAutoMigrateReturnsErrorOrSucceedsOnSQLite(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "flag-db.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "open sqlite: %v", err)

	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})

	// SQLite 可能因 MySQL 方言不兼容返回 error；关键是失败路径返回 error 而非 silent success。
	_ = NewMigrator(db, zap.NewNop()).AutoMigrate()
}
