package scheduler

import (
	"context"
	"f_ccg_video_back_ddd/pkg/constants"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/platform"

	domain "f_ccg_video_back_ddd/internal/domain/platform"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func TestDecayVideoHotScoreDecaysAndClears(t *testing.T) {
	t.Parallel()

	redisClient := openCronCacheTestRedis(t)
	ctx := context.Background()

	{
		err := redisClient.ZAdd(ctx, constants.VideoHotRankZSetKey,
			redis.Z{Member: "1", Score: 100},
			redis.Z{Member: "2", Score: domain.HotScoreClearThreshold / 2},
			redis.Z{Member: "3", Score: 50},
		).Err()
		require.NoError(t, err, "seed hot rank failed: %v", err)
	}

	app.NewHotRankDecayService(NewHotRankStore(redisClient), zap.NewNop()).Run(ctx)

	// 低于清理阈值的成员被移出榜单。
	{
		score, err := redisClient.ZScore(ctx, constants.VideoHotRankZSetKey, "2").Result()
		require.Error(t, err, "expected member 2 to be removed, got score %v", score)
	}

	for member, before := range map[string]float64{"1": 100, "3": 50} {
		score, err := redisClient.ZScore(ctx, constants.VideoHotRankZSetKey, member).Result()
		require.NoError(t, err, "load score of member %s failed: %v", member, err)

		require.InDelta(t, before*domain.HotScoreDecayFactor, score, 1e-9, "member %s: expected %v, got %v", member, before*domain.HotScoreDecayFactor, score)
	}
}

func TestDecayVideoHotScoreTrimsOverflow(t *testing.T) {
	t.Parallel()

	redisClient := openCronCacheTestRedis(t)
	ctx := context.Background()

	const overflow = 5

	members := make([]redis.Z, 0, domain.HotRankMaxSize+overflow)
	for i := 1; i <= domain.HotRankMaxSize+overflow; i++ {
		members = append(members, redis.Z{Member: strconv.Itoa(i), Score: float64(i)})
	}

	{
		err := redisClient.ZAdd(ctx, constants.VideoHotRankZSetKey, members...).Err()
		require.NoError(t, err, "seed hot rank failed: %v", err)
	}

	app.NewHotRankDecayService(NewHotRankStore(redisClient), zap.NewNop()).Run(ctx)

	total, err := redisClient.ZCard(ctx, constants.VideoHotRankZSetKey).Result()
	require.NoError(t, err, "load hot rank size failed: %v", err)

	require.EqualValues(t, domain.HotRankMaxSize, total, "expected rank trimmed to %d, got %d", domain.HotRankMaxSize, total)

	// 裁剪按分值从低到高，最低的 overflow 个成员应被移除。
	for i := 1; i <= overflow; i++ {
		{
			_, err := redisClient.ZScore(ctx, constants.VideoHotRankZSetKey, strconv.Itoa(i)).Result()
			require.Error(t, err, "expected lowest member %d to be trimmed", i)
		}
	}
}

func TestDecayVideoHotScoreSkipsWhenRedisMissing(t *testing.T) {
	t.Parallel()

	// Redis 未配置时只告警跳过，不 panic。
	app.NewHotRankDecayService(NewHotRankStore(nil), zap.NewNop()).Run(context.Background())
}
