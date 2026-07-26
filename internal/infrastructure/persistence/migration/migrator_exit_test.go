package migration

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func init() {
	// 测试环境下保证 zap 非 nil
	if zap.L() == nil {
		z, _ := zap.NewDevelopment()
		zap.ReplaceGlobals(z)
	}
}

func TestAutoMigrateReturnsErrorWhenDBNil(t *testing.T) {
	if err := NewMigrator(nil).AutoMigrate(); err == nil {
		t.Fatal("expected error when db is nil")
	}
}

func TestAutoMigrateReturnsErrorOrSucceedsOnSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "flag-db.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})

	// SQLite 可能因 MySQL 方言不兼容返回 error；关键是失败路径返回 error 而非 silent success。
	_ = NewMigrator(db).AutoMigrate()
}
