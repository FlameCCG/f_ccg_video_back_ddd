package video_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
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
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	statements := []string{
		`CREATE TABLE users (
			id integer PRIMARY KEY, created_at datetime, updated_at datetime,
			username text NOT NULL UNIQUE, email text, password text, avatar text,
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
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("prepare feed query schema: %v", err)
		}
	}
	if err := db.Exec(`INSERT INTO videos (
		id, title, author_username, partition_id, status, created_at
	) VALUES (1, 'feed video', 'feed-author', 1, ?, CURRENT_TIMESTAMP)`, enum.VideoStatusPublished).Error; err != nil {
		t.Fatalf("prepare feed video: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	return db
}

func TestFetchFeedJoinsAuthorAndSkipsEmptyFirstPageCount(t *testing.T) {
	db := openFeedQueryTestDB(t)
	query := infra.NewFeedQuery(db)

	var queryCount atomic.Int32
	if err := db.Callback().Query().Before("gorm:query").
		Register("test:count_feed_queries", func(*gorm.DB) {
			queryCount.Add(1)
		}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}

	items, total, err := query.FetchFeed(t.Context(), nil, nil, 1, 20)
	if err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}

	if total != 1 || len(items) != 1 || items[0].ID != 1 {
		t.Fatalf("feed = %+v total=%d, want video 1", items, total)
	}

	if items[0].Author.ID != 1 || items[0].Author.Avatar != "avatar.png" {
		t.Fatalf("joined author = %+v", items[0].Author)
	}

	if got := queryCount.Load(); got != 2 {
		t.Fatalf("non-empty first page query count = %d, want 2", got)
	}

	queryCount.Store(0)
	items, total, err = query.FetchFeed(t.Context(), []uint{999}, nil, 1, 20)
	if err != nil {
		t.Fatalf("FetchFeed empty: %v", err)
	}

	if total != 0 || len(items) != 0 {
		t.Fatalf("empty feed = %+v total=%d, want [] and 0", items, total)
	}

	if got := queryCount.Load(); got != 1 {
		t.Fatalf("empty first page query count = %d, want 1", got)
	}
}

func TestFeedInputsPropagateCanceledContext(t *testing.T) {
	db := openFeedQueryTestDB(t)
	history := infra.NewHistoryRepository(db)
	query := infra.NewFeedQuery(db)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := history.RecentVideoIDs(ctx, 1, 100); !errors.Is(err, context.Canceled) {
		t.Fatalf("history error = %v, want context.Canceled", err)
	}

	if _, err := query.PreferredPartitionIDs(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("preferences error = %v, want context.Canceled", err)
	}
}
