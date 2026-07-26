package migration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// VideoPartitionsSQLFile 分区种子 SQL 的默认路径（与原实现一致，相对工作目录）。
const VideoPartitionsSQLFile = "init_resource/sql/video_partitions.sql"

type videoPartitionSeedRow struct {
	SeedOrder     int    `gorm:"column:seed_order"`
	ID            uint   `gorm:"column:id"`
	Name          string `gorm:"column:name"`
	Icon          string `gorm:"column:icon"`
	IsActive      int    `gorm:"column:is_active"`
	IsSubmittable int    `gorm:"column:is_submittable"`
}

// SeedVideoPartitions 从 SQL 种子文件导入/更新视频分区，并把额外分区的 sort_order 重排到末尾。
//
// 返回 (导入条数, 被重排的额外分区数, 实际使用的种子文件路径, error)。
// 原实现为 flags.FlagVideoPartitionsInit（内部直接 os.Exit）；
// 退出码处理已上移到 internal/interfaces/cli。
func (m *Migrator) SeedVideoPartitions(ctx context.Context, sqlPath string) (int, int, string, error) {
	return initVideoPartitionsFromSQL(ctx, m.db, sqlPath)
}

func initVideoPartitionsFromSQL(ctx context.Context, db *gorm.DB, sqlPath string) (int, int, string, error) {
	if db == nil {
		return 0, 0, "", errors.New("数据库未初始化")
	}

	seedPath, err := resolveVideoPartitionsSQLPath(sqlPath)
	if err != nil {
		return 0, 0, "", err
	}

	content, err := os.ReadFile(seedPath)
	if err != nil {
		return 0, 0, "", fmt.Errorf("读取分区 SQL 文件失败: %w", err)
	}

	statements := splitSQLStatements(string(content))
	if len(statements) == 0 {
		return 0, 0, "", errors.New("分区 SQL 文件为空")
	}

	if err := db.WithContext(ctx).AutoMigrate(&po.VideoPartition{}); err != nil {
		return 0, 0, "", fmt.Errorf("迁移 video_partitions 表失败: %w", err)
	}

	var (
		imported         int
		normalizedExtras int
	)

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		tempTable := fmt.Sprintf("tmp_video_partition_seed_%d", time.Now().UnixNano())
		defer func() {
			_ = tx.Exec(dropVideoPartitionSeedTableSQL(tx.Name(), tempTable)).Error
		}()

		createTableSQL, err := createVideoPartitionSeedTableSQL(tx.Name(), tempTable)
		if err != nil {
			return err
		}

		if err := tx.Exec(createTableSQL).Error; err != nil {
			return fmt.Errorf("创建分区种子临时表失败: %w", err)
		}

		for _, stmt := range statements {
			rewritten, err := rewriteVideoPartitionSeedStatement(stmt, tempTable)
			if err != nil {
				return err
			}

			if err := tx.Exec(rewritten).Error; err != nil {
				return fmt.Errorf("执行分区种子 SQL 失败: %w", err)
			}
		}

		var seeds []videoPartitionSeedRow
		if err := tx.Table(tempTable).
			Order("seed_order ASC").
			Find(&seeds).Error; err != nil {
			return fmt.Errorf("读取分区种子数据失败: %w", err)
		}

		if len(seeds) == 0 {
			return errors.New("未解析到任何分区种子数据")
		}

		now := time.Now()
		partitionRows := make([]map[string]any, 0, len(seeds))

		seedIDs := make([]uint, 0, len(seeds))
		for idx, seed := range seeds {
			partitionRows = append(partitionRows, map[string]any{
				"id":             seed.ID,
				"created_at":     now,
				"updated_at":     now,
				"name":           seed.Name,
				"icon":           seed.Icon,
				"sort_order":     idx + 1,
				"is_active":      seed.IsActive != 0,
				"is_submittable": seed.IsSubmittable != 0,
			})
			seedIDs = append(seedIDs, seed.ID)
		}

		if err := tx.
			Table((&po.VideoPartition{}).TableName()).
			Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"name",
					"icon",
					"sort_order",
					"is_active",
					"is_submittable",
					"updated_at",
				}),
			}).
			Create(&partitionRows).Error; err != nil {
			return fmt.Errorf("写入分区数据失败: %w", err)
		}

		imported = len(partitionRows)

		var extraPartitions []po.VideoPartition
		if err := tx.
			Where("id NOT IN ?", seedIDs).
			Order("sort_order ASC, id ASC").
			Find(&extraPartitions).Error; err != nil {
			return fmt.Errorf("查询额外分区失败: %w", err)
		}

		nextSortOrder := imported + 1
		for _, partition := range extraPartitions {
			if err := tx.Model(&po.VideoPartition{}).
				Where("id = ?", partition.ID).
				Update("sort_order", nextSortOrder).Error; err != nil {
				return fmt.Errorf("重排额外分区排序失败: %w", err)
			}

			nextSortOrder++
		}

		normalizedExtras = len(extraPartitions)

		return nil
	})
	if err != nil {
		return 0, 0, "", err
	}

	return imported, normalizedExtras, seedPath, nil
}

func resolveVideoPartitionsSQLPath(path string) (string, error) {
	candidates := []string{path}
	if !filepath.IsAbs(path) {
		if cwd, err := os.Getwd(); err == nil {
			candidates = append(candidates, filepath.Join(cwd, path))
		}

		if executable, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(executable), path))
		}
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}

		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("未找到分区 SQL 文件: %s", path)
}

func splitSQLStatements(content string) []string {
	lines := strings.Split(content, "\n")

	statements := make([]string, 0, len(lines))
	for _, line := range lines {
		statement := strings.TrimSpace(line)
		if statement == "" || strings.HasPrefix(statement, "--") {
			continue
		}

		statements = append(statements, statement)
	}

	return statements
}

func rewriteVideoPartitionSeedStatement(statement, tempTable string) (string, error) {
	const sourceTable = "`video_partitions`"

	if !strings.Contains(statement, sourceTable) {
		return "", fmt.Errorf("分区 SQL 语句不合法: %s", statement)
	}

	return strings.Replace(statement, sourceTable, "`"+tempTable+"`", 1), nil
}

func createVideoPartitionSeedTableSQL(dialect, table string) (string, error) {
	quoted := quoteIdentifier(table)

	switch dialect {
	case "mysql":
		return "CREATE TEMPORARY TABLE " + quoted + ` (
seed_order BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
id BIGINT UNSIGNED NOT NULL,
created_at DATETIME(3) NULL,
updated_at DATETIME(3) NULL,
name VARCHAR(50) NOT NULL,
icon LONGTEXT NULL,
sort_order INT NOT NULL DEFAULT 0,
is_active TINYINT(1) NOT NULL DEFAULT 1,
is_submittable TINYINT(1) NOT NULL DEFAULT 1
)`, nil
	case "sqlite":
		return "CREATE TEMPORARY TABLE " + quoted + ` (
seed_order INTEGER PRIMARY KEY AUTOINCREMENT,
id INTEGER NOT NULL,
created_at DATETIME NULL,
updated_at DATETIME NULL,
name TEXT NOT NULL,
icon TEXT NULL,
sort_order INTEGER NOT NULL DEFAULT 0,
is_active INTEGER NOT NULL DEFAULT 1,
is_submittable INTEGER NOT NULL DEFAULT 1
)`, nil
	default:
		return "", fmt.Errorf("不支持的数据库方言: %s", dialect)
	}
}

func dropVideoPartitionSeedTableSQL(dialect, table string) string {
	if dialect == "mysql" {
		return "DROP TEMPORARY TABLE IF EXISTS " + quoteIdentifier(table)
	}

	return "DROP TABLE IF EXISTS " + quoteIdentifier(table)
}

func quoteIdentifier(name string) string {
	return "`" + name + "`"
}
