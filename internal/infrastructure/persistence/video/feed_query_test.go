package video_test

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openFeedQueryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "feed-query.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "open sqlite: %v", err)

	statements := []string{
		`CREATE TABLE users (
			id integer PRIMARY KEY, created_at datetime, updated_at datetime,
			username text NOT NULL UNIQUE, email text, password text, avatar text,
 refresh_token_hash text, must_change_password numeric,
			description text, gender integer, birthday datetime, level integer,
			open_id text, register_source text, exp integer, coin_count numeric,
			status integer, current_role_id integer, register_ip text, last_login_ip text,
			register_addr text, last_login_addr text
		)`,
		`CREATE TABLE videos (
			id integer PRIMARY KEY, created_at datetime, updated_at datetime,
			title text, description text, cover text, duration integer,
			author_username text, partition_id integer, views integer, likes integer,
			comment_count integer, coin_count integer, favorite_count integer,
			danmu_count integer, is_pinned numeric, pinned_at datetime,
			is_original numeric, storage_type integer, status integer, publish_at datetime,
			approved_at datetime, is_scheduled numeric NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE video_play_histories (
			user_id integer, video_id integer, progress integer, duration integer,
			created_at datetime, PRIMARY KEY (user_id, video_id)
		)`,
		`INSERT INTO users (id, username, avatar) VALUES (1, 'feed-author', 'avatar.png')`,
	}

	for _, statement := range statements {
		{
			err := db.Exec(statement).Error
			require.NoError(t, err, "prepare feed query schema: %v", err)
		}
	}

	{
		err := db.Exec(`INSERT INTO videos (
		id, title, author_username, partition_id, status, created_at
	) VALUES (1, 'feed video', 'feed-author', 1, ?, CURRENT_TIMESTAMP)`, enum.VideoStatusPublished).Error
		require.NoError(t, err, "prepare feed video: %v", err)
	}

	sqlDB, err := db.DB()
	require.NoError(t, err, "get sql db: %v", err)

	t.Cleanup(func() { _ = sqlDB.Close() })

	return db
}

func TestFetchFeedJoinsAuthorAndSkipsEmptyFirstPageCount(t *testing.T) {
	t.Parallel()

	db := openFeedQueryTestDB(t)
	query := infra.NewFeedQuery(db)

	var queryCount atomic.Int32

	{
		err := db.Callback().Query().Before("gorm:query").
			Register("test:count_feed_queries", func(*gorm.DB) {
				queryCount.Add(1)
			})
		require.NoError(t, err, "register query callback: %v", err)
	}

	items, total, err := query.FetchFeed(t.Context(), nil, nil, 1, 20)
	require.NoError(t, err, "FetchFeed: %v", err)

	require.EqualValues(t, 1, total, "feed = %+v total=%d, want video 1", items, total)
	require.Len(t, items, 1, "feed = %+v total=%d, want video 1", items, total)
	require.EqualValues(t, 1, items[0].ID, "feed = %+v total=%d, want video 1", items, total)

	require.EqualValues(t, 1, items[0].Author.ID, "joined author = %+v", items[0].Author)
	require.Equal(t, "avatar.png", items[0].Author.Avatar, "joined author = %+v", items[0].Author)

	{
		got := queryCount.Load()
		require.EqualValues(t, 2, got, "non-empty first page query count = %d, want 2", got)
	}

	queryCount.Store(0)

	items, total, err = query.FetchFeed(t.Context(), []uint{999}, nil, 1, 20)
	require.NoError(t, err, "FetchFeed empty: %v", err)

	require.EqualValues(t, 0, total, "empty feed = %+v total=%d, want [] and 0", items, total)
	require.Empty(t, items, "empty feed = %+v total=%d, want [] and 0", items, total)

	{
		got := queryCount.Load()
		require.EqualValues(t, 1, got, "empty first page query count = %d, want 1", got)
	}
}

func TestFeedInputsPropagateCanceledContext(t *testing.T) {
	t.Parallel()

	db := openFeedQueryTestDB(t)
	history := infra.NewHistoryRepository(db)
	query := infra.NewFeedQuery(db)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	{
		_, err := history.RecentVideoIDs(ctx, 1, 100)
		require.ErrorIs(t, err, context.Canceled, "history error = %v, want context.Canceled", err)
	}

	{
		_, err := query.PreferredPartitionIDs(ctx, 1)
		require.ErrorIs(t, err, context.Canceled, "preferences error = %v, want context.Canceled", err)
	}
}
