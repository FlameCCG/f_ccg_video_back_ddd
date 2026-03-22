package comment

import (
	"context"
	"errors"
	domain "f_ccg_video_back_ddd/internal/domain/comment"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"path/filepath"
	"testing"
)

func TestLikeCreateReturnsIdentityWithoutMutatingInputAndKeepsDuplicateError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "likes.db")), &gorm.Config{IgnoreRelationshipsWhenMigrating: true, DisableForeignKeyConstraintWhenMigrating: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&po.CommentLike{}); err != nil {
		t.Fatal(err)
	}
	repo := NewLikeRepository(db)
	input := &domain.Like{CommentID: 2, UserID: 3}
	created, err := repo.Create(context.Background(), input)
	if err != nil || created.ID == 0 || created.CreatedAt.IsZero() || input.ID != 0 || !input.CreatedAt.IsZero() {
		t.Fatal(created, input, err)
	}
	if _, err := repo.Create(context.Background(), input); !errors.Is(err, domain.ErrLikeDuplicated) {
		t.Fatal(err)
	}
}
