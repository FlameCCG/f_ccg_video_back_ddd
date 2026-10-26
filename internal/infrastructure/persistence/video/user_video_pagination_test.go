package video_test

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestUserVideoPaginationKeepsTotalOnLaterAndEmptyPages(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "pagination.db")), &gorm.Config{})
	require.NoError(t, err)

	conn, _ := db.DB()

	t.Cleanup(func() { _ = conn.Close() })

	{
		err := db.AutoMigrate(&po.Video{})
		require.NoError(t, err)
	}

	query := infra.NewUserVideoQuery(db)

	for i := range 45 {
		{
			err := db.Create(&po.Video{Title: strconv.Itoa(i), AuthorUsername: "author", Status: enum.VideoStatusPublished}).Error
			require.NoError(t, err)
		}
	}

	for _, page := range []int{1, 2, 3, 4} {
		rows, total, err := query.ListByAuthor(t.Context(), "author", false, 0, 0, page, 20)
		require.NoError(t, err)

		require.EqualValues(t, 45, total, "page=%d total=%d", page, total)

		want := 20
		if page == 3 {
			want = 5
		}

		if page == 4 {
			want = 0
		}

		require.Len(t, rows, want, "page=%d rows=%d", page, len(rows))
	}
}
