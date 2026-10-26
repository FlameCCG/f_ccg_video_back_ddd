package migration

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInitVideoPartitionsFromSQL(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "partitions.db")), &gorm.Config{})
	require.NoError(t, err, "open sqlite failed: %v", err)

	sqlDB, err := db.DB()
	require.NoError(t, err, "get sqlite db failed: %v", err)

	t.Cleanup(func() {
		{
			err := sqlDB.Close()
			require.NoError(t, err, "close sqlite db failed: %v", err)
		}
	})

	{
		err := db.AutoMigrate(&po.VideoPartition{})
		require.NoError(t, err, "auto migrate failed: %v", err)
	}

	existing := po.VideoPartition{
		Model:     po.Model{ID: 99},
		Name:      "额外分区",
		Icon:      "<svg>legacy</svg>",
		SortOrder: 1,
		IsActive:  true,
	}
	{
		err := db.Create(&existing).Error
		require.NoError(t, err, "create existing partition failed: %v", err)
	}

	outdatedSeed := po.VideoPartition{
		Model:     po.Model{ID: 2},
		Name:      "旧动画",
		Icon:      "<svg>old</svg>",
		SortOrder: 99,
		IsActive:  false,
	}
	{
		err := db.Create(&outdatedSeed).Error
		require.NoError(t, err, "create outdated seed partition failed: %v", err)
	}

	sqlPath := filepath.Join(t.TempDir(), "video_partitions.sql")

	sqlContent := "" +
		"INSERT INTO `video_partitions` (`id`, `created_at`, `updated_at`, `name`, `icon`, `sort_order`, `is_active`, `is_submittable`) VALUES (2, '2026-01-14 14:59:06.711', '2026-03-22 03:16:29.494', '动画', '<svg>anim</svg>', 9, 1, 1);\n" +
		"INSERT INTO `video_partitions` (`id`, `created_at`, `updated_at`, `name`, `icon`, `sort_order`, `is_active`, `is_submittable`) VALUES (3, '2026-01-14 14:59:06.711', '2026-03-22 03:16:29.494', '音乐', '<svg>music</svg>', 9, 0, 1);\n"
	{
		err := os.WriteFile(sqlPath, []byte(sqlContent), 0o600)
		require.NoError(t, err, "write sql file failed: %v", err)
	}

	imported, normalizedExtras, resolvedPath, err := initVideoPartitionsFromSQL(context.Background(), db, sqlPath)
	require.NoError(t, err, "init video partitions failed: %v", err)

	require.Equal(t, 2, imported, "unexpected imported count: got %d want %d", imported, 2)

	require.Equal(t, 1, normalizedExtras, "unexpected normalized extra count: got %d want %d", normalizedExtras, 1)

	require.Equal(t, resolvedPath, sqlPath, "unexpected resolved path: got %s want %s", resolvedPath, sqlPath)

	var partitions []po.VideoPartition
	{
		err := db.Order("sort_order ASC, id ASC").Find(&partitions).Error
		require.NoError(t, err, "query partitions failed: %v", err)
	}

	require.Len(t, partitions, 3, "unexpected partition count: got %d want %d", len(partitions), 3)

	require.EqualValues(t, 2, partitions[0].ID, "unexpected first partition: %+v", partitions[0])
	require.Equal(t, 1, partitions[0].SortOrder, "unexpected first partition: %+v", partitions[0])
	require.Equal(t, "动画", partitions[0].Name, "unexpected first partition: %+v", partitions[0])
	require.True(t, partitions[0].IsActive, "unexpected first partition: %+v", partitions[0])

	require.EqualValues(t, 3, partitions[1].ID, "unexpected second partition: %+v", partitions[1])
	require.Equal(t, 2, partitions[1].SortOrder, "unexpected second partition: %+v", partitions[1])
	require.Equal(t, "音乐", partitions[1].Name, "unexpected second partition: %+v", partitions[1])
	require.False(t, partitions[1].IsActive, "unexpected second partition: %+v", partitions[1])

	require.EqualValues(t, 99, partitions[2].ID, "unexpected third partition: %+v", partitions[2])
	require.Equal(t, 3, partitions[2].SortOrder, "unexpected third partition: %+v", partitions[2])
	require.Equal(t, "额外分区", partitions[2].Name, "unexpected third partition: %+v", partitions[2])
}
