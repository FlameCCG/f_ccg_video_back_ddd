package video_test

import (
	"encoding/json"
	"f_ccg_video_back_ddd/pkg/constants"
	"testing"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	infra "f_ccg_video_back_ddd/internal/infrastructure/cache/video"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// newStatsCacheFixture 用 miniredis 装配真实 StatsCache（测试不走 wire）。
func newStatsCacheFixture(t *testing.T) (*infra.StatsCache, *redis.Client) {
	t.Helper()

	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})

	t.Cleanup(func() {
		_ = client.Close()
	})

	return infra.NewStatsCache(client, zap.NewNop()), client
}

// TestApplyDeltaMarksDirtyAndAvoidsStaleOverwrite 覆盖原
// TestApplyVideoStatsDeltaMarksDirtyAndAvoidsStaleOverwrite：
// Lua 增量在 dirty 集合与 version 上留痕，且随后的 Seed（SETNX）不会用旧快照覆盖新值。
//
// 原测试还依赖 sqlite 提供「冷缓存回源 DB」的 base payload；新架构里 base 由应用层
// StatsService 组装后传入，因此这里直接给出等价的 base payload（同一组行内计数）。
func TestApplyDeltaMarksDirtyAndAvoidsStaleOverwrite(t *testing.T) {
	t.Parallel()

	cache, client := newStatsCacheFixture(t)
	ctx := t.Context()

	base := &domain.Stats{
		Views:         10,
		Likes:         5,
		CommentCount:  2,
		CoinCount:     1,
		FavoriteCount: 1,
		DanmuCount:    3,
	}

	basePayload, err := json.Marshal(base)
	require.NoError(t, err, "marshal base payload failed: %v", err)

	stats, err := cache.ApplyDelta(ctx, 1, domain.StatsDelta{
		Views:         2,
		Likes:         1,
		FavoriteCount: -1,
	}, string(basePayload))
	require.NoError(t, err, "apply stats delta failed: %v", err)

	require.NotNil(t, stats, "expected stats after delta")

	require.EqualValues(t, 12, stats.Views, "unexpected stats after delta: %+v", stats)
	require.EqualValues(t, 6, stats.Likes, "unexpected stats after delta: %+v", stats)
	require.EqualValues(t, 0, stats.FavoriteCount, "unexpected stats after delta: %+v", stats)

	dirty, err := client.SIsMember(ctx, constants.VideoDetailStatsDirtySetKey, "1").Result()
	require.NoError(t, err, "check dirty set failed: %v", err)

	require.True(t, dirty, "expected dirty member after delta")

	version, err := client.Get(ctx, constants.VideoDetailStatsVersionKeyPrefix+"1").Result()
	require.NoError(t, err, "get version failed: %v", err)

	require.Equal(t, "1", version, "expected version=1, got %s", version)

	// 回填旧快照时不应覆盖已经完成的增量更新（Seed 用 SETNX）。
	cache.Seed(ctx, 1, base)

	cached, ok := cache.Load(ctx, 1)
	require.True(t, ok, "expected cached stats")
	require.NotNil(t, cached, "expected cached stats")

	require.EqualValues(t, 12, cached.Views, "stale cache overwrite detected: %+v", cached)
	require.EqualValues(t, 6, cached.Likes, "stale cache overwrite detected: %+v", cached)
	require.EqualValues(t, 0, cached.FavoriteCount, "stale cache overwrite detected: %+v", cached)
}

// TestClearRemovesKeysAndDirtyMember 覆盖 Clear 的三 key 删除 + dirty 集合摘除。
func TestClearRemovesKeysAndDirtyMember(t *testing.T) {
	t.Parallel()

	cache, client := newStatsCacheFixture(t)
	ctx := t.Context()

	{
		_, err := cache.ApplyDelta(ctx, 7, domain.StatsDelta{Views: 1}, "")
		require.NoError(t, err, "apply stats delta failed: %v", err)
	}

	cache.Clear(ctx, 7)

	{
		_, ok := cache.Load(ctx, 7)
		require.False(t, ok, "expected stats key removed after clear")
	}

	if exists, err := client.Exists(ctx, constants.VideoDetailStatsVersionKeyPrefix+"7").Result(); err != nil {
		t.Fatalf("check version key failed: %v", err)
	} else {
		require.EqualValues(t, 0, exists, "expected version key removed after clear")
	}

	dirty, err := client.SIsMember(ctx, constants.VideoDetailStatsDirtySetKey, "7").Result()
	require.NoError(t, err, "check dirty set failed: %v", err)

	require.False(t, dirty, "expected dirty member removed after clear")
}

// TestApplyDeltaIfExistsSkipsColdCache 覆盖「冷缓存不 patch」语义。
func TestApplyDeltaIfExistsSkipsColdCache(t *testing.T) {
	t.Parallel()

	cache, _ := newStatsCacheFixture(t)
	ctx := t.Context()

	stats, existed, err := cache.ApplyDeltaIfExists(ctx, 42, domain.StatsDelta{FavoriteCount: -1})
	require.NoError(t, err, "apply if-exists failed: %v", err)

	require.False(t, existed, "expected cold cache miss, got existed=%v stats=%+v", existed, stats)
	require.Nil(t, stats, "expected cold cache miss, got existed=%v stats=%+v", existed, stats)

	{
		_, err := cache.ApplyDelta(ctx, 42, domain.StatsDelta{FavoriteCount: 3}, "")
		require.NoError(t, err, "seed via delta failed: %v", err)
	}

	stats, existed, err = cache.ApplyDeltaIfExists(ctx, 42, domain.StatsDelta{FavoriteCount: -1})
	require.NoError(t, err, "apply if-exists failed: %v", err)

	require.True(t, existed, "expected hot cache patch, got existed=%v stats=%+v", existed, stats)
	require.NotNil(t, stats, "expected hot cache patch, got existed=%v stats=%+v", existed, stats)

	require.EqualValues(t, 2, stats.FavoriteCount, "unexpected favorite count: %+v", stats)
}
