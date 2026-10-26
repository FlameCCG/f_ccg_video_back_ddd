package scheduler

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"f_ccg_video_back_ddd/pkg/constants"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/platform"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openCronCacheTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cron-cache-test.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "open sqlite failed: %v", err)

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
		{
			err := db.Exec(stmt).Error
			require.NoError(t, err, "create test tables failed: %v", err)
		}
	}

	sqlDB, err := db.DB()
	require.NoError(t, err, "get sql db failed: %v", err)

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
	t.Parallel()

	db := openCronCacheTestDB(t)
	redisClient := openCronCacheTestRedis(t)

	{
		err := db.Exec(
			`INSERT INTO videos (id, title, author_username, status) VALUES (?, ?, ?, ?)`,
			7, "dirty", "alice", enum.VideoStatusPublished,
		).Error
		require.NoError(t, err, "create video failed: %v", err)
	}

	payload := `{"views":101,"likes":6,"commentCount":4,"coinCount":3,"favoriteCount":2,"danmuCount":9}`
	{
		err := redisClient.Set(context.Background(), constants.VideoDetailStatsKeyPrefix+"7", payload, 0).Err()
		require.NoError(t, err, "seed redis stats failed: %v", err)
	}

	{
		err := redisClient.Set(context.Background(), constants.VideoDetailStatsVersionKeyPrefix+"7", 1, 0).Err()
		require.NoError(t, err, "seed redis version failed: %v", err)
	}

	{
		err := redisClient.SAdd(context.Background(), constants.VideoDetailStatsDirtySetKey, "7").Err()
		require.NoError(t, err, "seed dirty set failed: %v", err)
	}

	service := app.NewVideoStatSyncService(
		NewVideoStatsCache(redisClient, zap.NewNop()),
		NewVideoStatsProjection(db),
		zap.NewNop(),
	)
	service.Run(context.Background())

	var updated po.Video
	{
		err := db.Select("views", "likes", "comment_count", "coin_count", "favorite_count", "danmu_count").
			Where("id = ?", 7).Take(&updated).Error
		require.NoError(t, err, "load updated video failed: %v", err)
	}

	require.EqualValues(t, 101, updated.Views, "unexpected synced video stats: %+v", updated)
	require.EqualValues(t, 6, updated.Likes, "unexpected synced video stats: %+v", updated)
	require.EqualValues(t, 4, updated.CommentCount, "unexpected synced video stats: %+v", updated)
	require.EqualValues(t, 3, updated.CoinCount, "unexpected synced video stats: %+v", updated)
	require.EqualValues(t, 2, updated.FavoriteCount, "unexpected synced video stats: %+v", updated)
	require.EqualValues(t, 9, updated.DanmuCount, "unexpected synced video stats: %+v", updated)

	dirty, err := redisClient.SIsMember(context.Background(), constants.VideoDetailStatsDirtySetKey, "7").Result()
	require.NoError(t, err, "check dirty set failed: %v", err)

	require.False(t, dirty, "expected dirty member to be cleared after sync")
}

func TestAckDirtyVideoStatsVersionKeepsDirtyOnVersionMismatch(t *testing.T) {
	t.Parallel()

	redisClient := openCronCacheTestRedis(t)

	{
		err := redisClient.SAdd(context.Background(), constants.VideoDetailStatsDirtySetKey, "8").Err()
		require.NoError(t, err, "seed dirty set failed: %v", err)
	}

	{
		err := redisClient.Set(context.Background(), constants.VideoDetailStatsVersionKeyPrefix+"8", 2, 0).Err()
		require.NoError(t, err, "seed redis version failed: %v", err)
	}

	cache := NewVideoStatsCache(redisClient, zap.NewNop())
	{
		err := cache.AckVersion(context.Background(), "8", 1)
		require.NoError(t, err, "ack dirty version failed: %v", err)
	}

	dirty, err := redisClient.SIsMember(context.Background(), constants.VideoDetailStatsDirtySetKey, "8").Result()
	require.NoError(t, err, "check dirty set failed: %v", err)

	require.True(t, dirty, "expected dirty member to remain when version mismatches")
}

func TestSyncCommentLikeRecoversProcessingQueue(t *testing.T) {
	t.Parallel()

	db := openCronCacheTestDB(t)
	redisClient := openCronCacheTestRedis(t)

	{
		err := db.Exec(
			`INSERT INTO comments (id, content, user_id, video_id, dynamic_id, comment_type, like_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			11, "hello", 1, 0, 0, 1, 0,
		).Error
		require.NoError(t, err, "create comment failed: %v", err)
	}

	event := `{"commentId":11,"userId":21,"action":"like"}`
	{
		err := redisClient.RPush(context.Background(), constants.CommentLikeProcessingQueueKey, event).Err()
		require.NoError(t, err, "seed processing queue failed: %v", err)
	}

	service := app.NewCommentLikeSyncService(
		gormx.NewTxManager(db),
		NewCommentLikeQueue(redisClient),
		NewCommentLikeProjection(db),
		zap.NewNop(),
	)
	service.Run(context.Background())

	var count int64
	{
		err := db.Model(&po.CommentLike{}).
			Where("comment_id = ? AND user_id = ?", 11, 21).
			Count(&count).Error
		require.NoError(t, err, "count comment likes failed: %v", err)
	}

	require.EqualValues(t, 1, count, "expected one comment like row, got %d", count)

	var updated po.Comment
	{
		err := db.Select("like_count").Where("id = ?", 11).Take(&updated).Error
		require.NoError(t, err, "load comment failed: %v", err)
	}

	require.EqualValues(t, 1, updated.LikeCount, "expected like_count=1, got %d", updated.LikeCount)

	processingLen, err := redisClient.LLen(context.Background(), constants.CommentLikeProcessingQueueKey).Result()
	require.NoError(t, err, "check processing queue failed: %v", err)

	require.EqualValues(t, 0, processingLen, "expected empty processing queue, got %d", processingLen)
}
