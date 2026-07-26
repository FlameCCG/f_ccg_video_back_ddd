package scheduler

import (
	"context"
	"path/filepath"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/platform"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openCronCacheTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cron-cache-test.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	statements := []string{
		`CREATE TABLE videos (
			id integer PRIMARY KEY AUTOINCREMENT,
			created_at datetime,
			updated_at datetime,
			title text,
			author_username text,
			views integer DEFAULT 0,
			likes integer DEFAULT 0,
			comment_count integer DEFAULT 0,
			coin_count integer DEFAULT 0,
			favorite_count integer DEFAULT 0,
			danmu_count integer DEFAULT 0,
			status integer DEFAULT 1
		)`,
		`CREATE TABLE comments (
			id integer PRIMARY KEY AUTOINCREMENT,
			created_at datetime,
			updated_at datetime,
			content text,
			user_id integer DEFAULT 0,
			video_id integer DEFAULT 0,
			dynamic_id integer DEFAULT 0,
			comment_type integer DEFAULT 1,
			like_count integer DEFAULT 0
		)`,
		`CREATE TABLE comment_likes (
			id integer PRIMARY KEY AUTOINCREMENT,
			created_at datetime,
			updated_at datetime,
			comment_id integer NOT NULL,
			user_id integer NOT NULL,
			UNIQUE(comment_id, user_id)
		)`,
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create test tables failed: %v", err)
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db failed: %v", err)
	}

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return db
}

func openCronCacheTestRedis(t *testing.T) *redis.Client {
	t.Helper()

	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})

	t.Cleanup(func() {
		_ = client.Close()
	})

	return client
}

func TestSyncVideoStatUpdatesOnlyDirtyEntries(t *testing.T) {
	db := openCronCacheTestDB(t)
	redisClient := openCronCacheTestRedis(t)

	if err := db.Exec(
		`INSERT INTO videos (id, title, author_username, status) VALUES (?, ?, ?, ?)`,
		7, "dirty", "alice", enum.VideoStatusPublished,
	).Error; err != nil {
		t.Fatalf("create video failed: %v", err)
	}

	payload := `{"views":101,"likes":6,"commentCount":4,"coinCount":3,"favoriteCount":2,"danmuCount":9}`
	if err := redisClient.Set(context.Background(), constants.VideoDetailStatsKeyPrefix+"7", payload, 0).Err(); err != nil {
		t.Fatalf("seed redis stats failed: %v", err)
	}

	if err := redisClient.Set(context.Background(), constants.VideoDetailStatsVersionKeyPrefix+"7", 1, 0).Err(); err != nil {
		t.Fatalf("seed redis version failed: %v", err)
	}

	if err := redisClient.SAdd(context.Background(), constants.VideoDetailStatsDirtySetKey, "7").Err(); err != nil {
		t.Fatalf("seed dirty set failed: %v", err)
	}

	service := app.NewVideoStatSyncService(NewVideoStatsCache(redisClient), NewVideoStatsProjection(db))
	service.Run(context.Background())

	var updated po.Video
	if err := db.Select("views", "likes", "comment_count", "coin_count", "favorite_count", "danmu_count").
		Where("id = ?", 7).Take(&updated).Error; err != nil {
		t.Fatalf("load updated video failed: %v", err)
	}

	if updated.Views != 101 || updated.Likes != 6 || updated.CommentCount != 4 ||
		updated.CoinCount != 3 || updated.FavoriteCount != 2 || updated.DanmuCount != 9 {
		t.Fatalf("unexpected synced video stats: %+v", updated)
	}

	dirty, err := redisClient.SIsMember(context.Background(), constants.VideoDetailStatsDirtySetKey, "7").Result()
	if err != nil {
		t.Fatalf("check dirty set failed: %v", err)
	}

	if dirty {
		t.Fatal("expected dirty member to be cleared after sync")
	}
}

func TestAckDirtyVideoStatsVersionKeepsDirtyOnVersionMismatch(t *testing.T) {
	redisClient := openCronCacheTestRedis(t)

	if err := redisClient.SAdd(context.Background(), constants.VideoDetailStatsDirtySetKey, "8").Err(); err != nil {
		t.Fatalf("seed dirty set failed: %v", err)
	}

	if err := redisClient.Set(context.Background(), constants.VideoDetailStatsVersionKeyPrefix+"8", 2, 0).Err(); err != nil {
		t.Fatalf("seed redis version failed: %v", err)
	}

	cache := NewVideoStatsCache(redisClient)
	if err := cache.AckVersion(context.Background(), "8", 1); err != nil {
		t.Fatalf("ack dirty version failed: %v", err)
	}

	dirty, err := redisClient.SIsMember(context.Background(), constants.VideoDetailStatsDirtySetKey, "8").Result()
	if err != nil {
		t.Fatalf("check dirty set failed: %v", err)
	}

	if !dirty {
		t.Fatal("expected dirty member to remain when version mismatches")
	}
}

func TestSyncCommentLikeRecoversProcessingQueue(t *testing.T) {
	db := openCronCacheTestDB(t)
	redisClient := openCronCacheTestRedis(t)

	if err := db.Exec(
		`INSERT INTO comments (id, content, user_id, video_id, dynamic_id, comment_type, like_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		11, "hello", 1, 0, 0, 1, 0,
	).Error; err != nil {
		t.Fatalf("create comment failed: %v", err)
	}

	event := `{"commentId":11,"userId":21,"action":"like"}`
	if err := redisClient.RPush(context.Background(), constants.CommentLikeProcessingQueueKey, event).Err(); err != nil {
		t.Fatalf("seed processing queue failed: %v", err)
	}

	service := app.NewCommentLikeSyncService(
		gormx.NewTxManager(db),
		NewCommentLikeQueue(redisClient),
		NewCommentLikeProjection(db),
	)
	service.Run(context.Background())

	var count int64
	if err := db.Model(&po.CommentLike{}).
		Where("comment_id = ? AND user_id = ?", 11, 21).
		Count(&count).Error; err != nil {
		t.Fatalf("count comment likes failed: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected one comment like row, got %d", count)
	}

	var updated po.Comment
	if err := db.Select("like_count").Where("id = ?", 11).Take(&updated).Error; err != nil {
		t.Fatalf("load comment failed: %v", err)
	}

	if updated.LikeCount != 1 {
		t.Fatalf("expected like_count=1, got %d", updated.LikeCount)
	}

	processingLen, err := redisClient.LLen(context.Background(), constants.CommentLikeProcessingQueueKey).Result()
	if err != nil {
		t.Fatalf("check processing queue failed: %v", err)
	}

	if processingLen != 0 {
		t.Fatalf("expected empty processing queue, got %d", processingLen)
	}
}
