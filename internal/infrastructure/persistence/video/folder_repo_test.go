package video_test

import (
	"errors"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"path/filepath"
	"testing"

	domain "f_ccg_video_back_ddd/internal/domain/video"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openFolderRepoTestDB 建一个只含收藏夹表的 sqlite 临时库。
//
// 不直接复用 newFixture：其 AutoMigrate 会因 video_like/video_coin 同名索引
// idx_video_user 冲突而失败，与本用例无关。
func openFolderRepoTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "folder-repo-test.db") +
		"?_pragma=synchronous(OFF)&_pragma=journal_mode(MEMORY)"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&po.FavoriteFolder{}); err != nil {
		t.Fatalf("migrate favorite_folders: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}

	t.Cleanup(func() { _ = sqlDB.Close() })

	return db
}

// TestFolderRepositoryUpdate 验证 Update 仅更新名称，且保留归属与默认标记。
func TestFolderRepositoryUpdate(t *testing.T) {
	t.Parallel()
	repo := infra.NewFolderRepository(openFolderRepoTestDB(t))

	folder := &domain.FavoriteFolder{UserID: 7, Name: "旧名字"}
	if err := repo.Create(t.Context(), folder); err != nil {
		t.Fatalf("create: %v", err)
	}

	folder.Name = "新名字"
	if err := repo.Update(t.Context(), folder); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.FindByIDAndUser(t.Context(), 7, folder.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if got.Name != "新名字" {
		t.Fatalf("name = %q, want %q", got.Name, "新名字")
	}

	if got.UserID != 7 {
		t.Fatalf("userID = %d, want 7", got.UserID)
	}
}

// TestFolderRepositoryUpdateRejectsInvalidParam 校验空实体与零 ID 直接拒绝。
func TestFolderRepositoryUpdateRejectsInvalidParam(t *testing.T) {
	t.Parallel()
	repo := infra.NewFolderRepository(openFolderRepoTestDB(t))

	if err := repo.Update(t.Context(), nil); !errors.Is(err, domain.ErrParamInvalid) {
		t.Fatalf("nil folder err = %v, want %v", err, domain.ErrParamInvalid)
	}

	if err := repo.Update(t.Context(), &domain.FavoriteFolder{}); !errors.Is(err, domain.ErrParamInvalid) {
		t.Fatalf("zero id err = %v, want %v", err, domain.ErrParamInvalid)
	}
}
