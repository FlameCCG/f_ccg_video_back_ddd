// Package mysql 构造和关闭 MySQL/GORM 连接池。
//
// mysql.go 读取连接、时区、日志和池参数并暴露供仓储使用的 GORM 句柄。
// 连接构造失败会通过 zap.Panic 中断，不能当作普通 Wire error 返回；成功后由 cleanup 或 Infrastructure.Stop 关闭。
// 业务事务另由 gormx 管理。
//
// provider.go 只导出构造器与窄化访问能力，不应在业务用例中再次创建全局连接。
package mysql

import (
	"time"

	"f_ccg_video_back_ddd/internal/config"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	maxOpenConns    = 100
	maxIdleConns    = 10
	connMaxLifetime = time.Hour
	// defaultContextTimeout 与原实现一致，防止无 ctx 的语句无限阻塞。
	defaultContextTimeout = 20 * time.Second
)

// DB 包装 *gorm.DB，负责连接池生命周期。
type DB struct {
	db     *gorm.DB
	logger *zap.Logger
}

// Gorm 返回底层 *gorm.DB。
func (d *DB) Gorm() *gorm.DB {
	if d == nil {
		return nil
	}

	return d.db
}

// New 连接 MySQL 并配置连接池；失败时 zap.Panic（与原 core.InitDB 一致）。
func New(cfg *config.Mysql, logger *zap.Logger) *DB {
	db, err := gorm.Open(mysql.Open(cfg.GetDSN()), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		DefaultContextTimeout:                    defaultContextTimeout,
	})
	if err != nil {
		logger.Panic("连接数据库失败", zap.Error(err))
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.Panic("获取数据库连接失败", zap.Error(err))
	}

	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	logger.Info("mysql连接成功", zap.String("database", cfg.DBName))

	return &DB{db: db, logger: logger}
}

// Stop 关闭 MySQL 连接池。
func (d *DB) Stop() {
	if d == nil || d.db == nil {
		return
	}

	sqlDB, err := d.db.DB()
	if err != nil {
		d.logger.Warn("获取数据库连接失败", zap.Error(err))

		return
	}

	if err := sqlDB.Close(); err != nil {
		d.logger.Warn("mysql close failed", zap.Error(err))

		return
	}

	d.db = nil

	d.logger.Info("mysql连接已关闭")
}

// ProvideGorm 从 DB 包装中取出 *gorm.DB 供仓储注入。
func ProvideGorm(d *DB) *gorm.DB {
	return d.Gorm()
}

// Provide 供 wire 使用的 provider：返回连接池与其资源关闭函数。
func Provide(cfg *config.Mysql, logger *zap.Logger) (*DB, func()) {
	db := New(cfg, logger)

	return db, db.Stop
}
