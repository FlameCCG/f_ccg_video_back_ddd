package migration

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestInitVideoPartitionsFromSQL(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "partitions.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite db failed: %v", err)
	}

	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("close sqlite db failed: %v", err)
		}
	})

	if err := db.AutoMigrate(&po.VideoPartition{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	existing := po.VideoPartition{
		Model:     po.Model{ID: 99},
		Name:      "额外分区",
		Icon:      "<svg>legacy</svg>",
		SortOrder: 1,
		IsActive:  true,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing partition failed: %v", err)
	}

	outdatedSeed := po.VideoPartition{
		Model:     po.Model{ID: 2},
		Name:      "旧动画",
		Icon:      "<svg>old</svg>",
		SortOrder: 99,
		IsActive:  false,
	}
	if err := db.Create(&outdatedSeed).Error; err != nil {
		t.Fatalf("create outdated seed partition failed: %v", err)
	}

	sqlPath := filepath.Join(t.TempDir(), "video_partitions.sql")

	sqlContent := "" +
		"INSERT INTO `video_partitions` (`id`, `created_at`, `updated_at`, `name`, `icon`, `sort_order`, `is_active`, `is_submittable`) VALUES (2, '2026-01-14 14:59:06.711', '2026-03-22 03:16:29.494', '动画', '<svg>anim</svg>', 9, 1, 1);\n" +
		"INSERT INTO `video_partitions` (`id`, `created_at`, `updated_at`, `name`, `icon`, `sort_order`, `is_active`, `is_submittable`) VALUES (3, '2026-01-14 14:59:06.711', '2026-03-22 03:16:29.494', '音乐', '<svg>music</svg>', 9, 0, 1);\n"
	if err := os.WriteFile(sqlPath, []byte(sqlContent), 0o644); err != nil {
		t.Fatalf("write sql file failed: %v", err)
	}

	imported, normalizedExtras, resolvedPath, err := initVideoPartitionsFromSQL(context.Background(), db, sqlPath)
	if err != nil {
		t.Fatalf("init video partitions failed: %v", err)
	}

	if imported != 2 {
		t.Fatalf("unexpected imported count: got %d want %d", imported, 2)
	}

	if normalizedExtras != 1 {
		t.Fatalf("unexpected normalized extra count: got %d want %d", normalizedExtras, 1)
	}

	if resolvedPath != sqlPath {
		t.Fatalf("unexpected resolved path: got %s want %s", resolvedPath, sqlPath)
	}

	var partitions []po.VideoPartition
	if err := db.Order("sort_order ASC, id ASC").Find(&partitions).Error; err != nil {
		t.Fatalf("query partitions failed: %v", err)
	}

	if len(partitions) != 3 {
		t.Fatalf("unexpected partition count: got %d want %d", len(partitions), 3)
	}

	if partitions[0].ID != 2 || partitions[0].SortOrder != 1 || partitions[0].Name != "动画" || !partitions[0].IsActive {
		t.Fatalf("unexpected first partition: %+v", partitions[0])
	}

	if partitions[1].ID != 3 || partitions[1].SortOrder != 2 || partitions[1].Name != "音乐" || partitions[1].IsActive {
		t.Fatalf("unexpected second partition: %+v", partitions[1])
	}

	if partitions[2].ID != 99 || partitions[2].SortOrder != 3 || partitions[2].Name != "额外分区" {
		t.Fatalf("unexpected third partition: %+v", partitions[2])
	}
}
