package video_test

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"
	"fmt"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func TestUserVideoPaginationKeepsTotalOnLaterAndEmptyPages(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "pagination.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.AutoMigrate(&po.Video{}); err != nil {
		t.Fatal(err)
	}
	query := infra.NewUserVideoQuery(db)
	for i := 0; i < 45; i++ {
		if err := db.Create(&po.Video{Title: fmt.Sprint(i), AuthorUsername: "author", Status: enum.VideoStatusPublished}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, page := range []int{1, 2, 3, 4} {
		rows, total, err := query.ListByAuthor(t.Context(), "author", false, 0, 0, page, 20)
		if err != nil {
			t.Fatal(err)
		}
		if total != 45 {
			t.Fatalf("page=%d total=%d", page, total)
		}
		want := 20
		if page == 3 {
			want = 5
		}
		if page == 4 {
			want = 0
		}
		if len(rows) != want {
			t.Fatalf("page=%d rows=%d", page, len(rows))
		}
	}
}
