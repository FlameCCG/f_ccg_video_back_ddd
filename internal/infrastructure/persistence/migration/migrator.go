// Package migration 实现MySQL 表结构迁移和视频分区初始化。
//
// migrator.go 枚举需要迁移的 PO，video_partitions.go 导入分区数据；入口来自一次性 CLI 任务。
// AutoMigrate 的结构调整与业务数据迁移是不同工作，不能仅凭新增字段就假定旧数据已得到业务正确的值。
//
// 仓储遵守消费方端口的不存在、错误和 ID 回填约定，不能只看方法名判断 nil 或错误的含义。
package migration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Migrator 数据库迁移器。
//
// 原实现为 flags 包里的包级函数直读 global.DB；新架构改为注入 *gorm.DB 的实例。
type Migrator struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewMigrator 构造数据库迁移器。
func NewMigrator(db *gorm.DB, logger *zap.Logger) *Migrator {
	return &Migrator{db: db, logger: logger}
}

// AutoMigrate 执行数据库迁移；失败返回 error，由调用方以非 0 退出码结束进程。
// 迁移的实体集合与顺序与原 flags.FlagDB 完全一致。
func (m *Migrator) AutoMigrate() error {
	if m == nil || m.db == nil {
		err := fmt.Errorf("数据库未初始化")
		m.logger.Error("数据库迁移失败!", zap.Error(err))

		return err
	}

	err := m.db.AutoMigrate(
		po.User{},
		po.UserConf{},
		po.UserBanRecord{},
		po.UserFocus{},
		po.UserLoginRecord{},
		po.UserExpRecord{},
		po.UserCoinRecord{},
		po.Comment{},
		po.VideoReport{},
		po.DanmuReport{},
		po.Banner{},
		po.UserGlobalNotification{},
		po.NotificationUserRead{},
		po.Video{},
		po.VideoPart{},
		po.VideoPartResource{},
		po.VideoPlayHistory{},
		po.VideoPartition{},
		po.VideoResource{},
		po.VideoLike{},
		po.VideoCoin{},
		po.FavoriteFolder{},
		po.FavoriteFolderVideo{},
		po.Danmu{},
		po.DanmuLike{},
		po.Tag{},
		po.Dynamic{},
		po.DynamicLike{},
		po.DynamicRead{},
		po.CommentLike{},
		po.Role{},
		po.Menu{},
		po.UserRole{},
		po.RoleMenu{},
		po.CasbinRule{},
		po.ApiResource{},
	)
	if err != nil {
		m.logger.Error("数据库迁移失败!", zap.Error(err))

		return fmt.Errorf("数据库迁移失败: %w", err)
	}

	if err := ensureUsersOAuthUniqueIndex(m.db); err != nil {
		m.logger.Error("创建 OAuth 唯一索引失败!", zap.Error(err))

		return fmt.Errorf("创建 OAuth 唯一索引失败: %w", err)
	}

	if err := migrateLegacySchedules(m.db); err != nil {
		return fmt.Errorf("迁移旧定时计划失败: %w", err)
	}
	m.logger.Info("数据库迁移成功!")

	return nil
}

// ensureUsersOAuthUniqueIndex 保证 (open_id, register_source) 在 open_id 非空时唯一。
//
// 使用普通列唯一索引，而不是 MySQL 8 函数索引：
// go-mysql canal（River）解析不了函数索引 DDL，会直接中断 binlog 同步。
// 空 open_id（邮箱注册）规范为 NULL；MySQL 唯一索引允许多行 NULL，不会互相冲突。
func ensureUsersOAuthUniqueIndex(db *gorm.DB) error {
	if db == nil {
		return errors.New("db is nil")
	}

	const indexName = "uk_users_open_id_source"

	// SQLite 等测试库：跳过（无 information_schema / 函数索引语义）。
	if isSQLiteDialector(db) {
		return nil
	}

	// 先把历史空串规范为 NULL，再考虑建普通唯一索引。
	if err := db.Exec(`UPDATE users SET open_id = NULL WHERE open_id = ''`).Error; err != nil {
		if isIgnorableIndexProbeError(err) {
			return nil
		}

		return fmt.Errorf("normalize empty open_id to null: %w", err)
	}

	indexKind, err := probeUsersOAuthUniqueIndex(db, indexName)
	if err != nil {
		if isIgnorableIndexProbeError(err) {
			return nil
		}

		return err
	}

	switch indexKind {
	case oauthIndexPlain:
		return nil
	case oauthIndexFunctional:
		// 旧版函数索引会写到 binlog 并打挂 River，迁移为普通列索引。
		if err := db.Exec("DROP INDEX `" + indexName + "` ON `users`").Error; err != nil {
			return fmt.Errorf("drop functional oauth unique index: %w", err)
		}
	}

	err = db.Exec(`
		CREATE UNIQUE INDEX uk_users_open_id_source ON users (open_id, register_source)
	`).Error
	if err != nil && isDuplicateIndexError(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("create oauth unique index: %w", err)
	}

	return nil
}

const (
	oauthIndexMissing    = ""
	oauthIndexPlain      = "plain"
	oauthIndexFunctional = "functional"
)

// probeUsersOAuthUniqueIndex 判断 uk_users_open_id_source 是否存在，以及是否为函数索引。
func probeUsersOAuthUniqueIndex(db *gorm.DB, indexName string) (string, error) {
	type indexRow struct {
		ColumnName string  `gorm:"column:column_name"`
		Expression *string `gorm:"column:expression"`
	}

	var rows []indexRow
	// MySQL 8：函数索引的 column_name 为空，expression 非空。
	if err := db.Raw(`
		SELECT column_name, expression
		FROM information_schema.statistics
		WHERE table_schema = DATABASE()
		  AND table_name = 'users'
		  AND index_name = ?
	`, indexName).Scan(&rows).Error; err != nil {
		return oauthIndexMissing, err
	}

	if len(rows) == 0 {
		return oauthIndexMissing, nil
	}

	for _, row := range rows {
		if row.Expression != nil && strings.TrimSpace(*row.Expression) != "" {
			return oauthIndexFunctional, nil
		}

		if strings.TrimSpace(row.ColumnName) == "" {
			return oauthIndexFunctional, nil
		}
	}

	return oauthIndexPlain, nil
}

func isSQLiteDialector(db *gorm.DB) bool {
	if db == nil || db.Dialector == nil {
		return false
	}

	name := strings.ToLower(db.Dialector.Name())

	return name == "sqlite" || strings.Contains(name, "sqlite")
}

func isIgnorableIndexProbeError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "information_schema") ||
		strings.Contains(msg, "doesn't exist")
}

func isDuplicateIndexError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "duplicate key name") ||
		strings.Contains(msg, "already exists")
}

// MigrateBannerTable 单独迁移 banner 表（Banner 初始化任务在写入前调用）。
func (m *Migrator) MigrateBannerTable(ctx context.Context) error {
	return m.db.WithContext(ctx).AutoMigrate(&po.Banner{})
}
