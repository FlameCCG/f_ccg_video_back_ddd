package scheduler

import (
	"context"
	"strconv"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/platform"
	domain "f_ccg_video_back_ddd/internal/domain/platform"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func TestDecayVideoHotScoreDecaysAndClears(t *testing.T) {
	redisClient := openCronCacheTestRedis(t)
	ctx := context.Background()

	if err := redisClient.ZAdd(ctx, constants.VideoHotRankZSetKey,
		redis.Z{Member: "1", Score: 100},
		redis.Z{Member: "2", Score: domain.HotScoreClearThreshold / 2},
		redis.Z{Member: "3", Score: 50},
	).Err(); err != nil {
		t.Fatalf("seed hot rank failed: %v", err)
	}

	app.NewHotRankDecayService(NewHotRankStore(redisClient), zap.NewNop()).Run(ctx)

	// 低于清理阈值的成员被移出榜单。
	if score, err := redisClient.ZScore(ctx, constants.VideoHotRankZSetKey, "2").Result(); err == nil {
		t.Fatalf("expected member 2 to be removed, got score %v", score)
	}

	for member, before := range map[string]float64{"1": 100, "3": 50} {
		score, err := redisClient.ZScore(ctx, constants.VideoHotRankZSetKey, member).Result()
		if err != nil {
			t.Fatalf("load score of member %s failed: %v", member, err)
		}

		if score != before*domain.HotScoreDecayFactor {
			t.Fatalf("member %s: expected %v, got %v", member, before*domain.HotScoreDecayFactor, score)
		}
	}
}

func TestDecayVideoHotScoreTrimsOverflow(t *testing.T) {
	redisClient := openCronCacheTestRedis(t)
	ctx := context.Background()

	const overflow = 5

	members := make([]redis.Z, 0, domain.HotRankMaxSize+overflow)
	for i := 1; i <= domain.HotRankMaxSize+overflow; i++ {
		members = append(members, redis.Z{Member: strconv.Itoa(i), Score: float64(i)})
	}

	if err := redisClient.ZAdd(ctx, constants.VideoHotRankZSetKey, members...).Err(); err != nil {
		t.Fatalf("seed hot rank failed: %v", err)
	}

	app.NewHotRankDecayService(NewHotRankStore(redisClient), zap.NewNop()).Run(ctx)

	total, err := redisClient.ZCard(ctx, constants.VideoHotRankZSetKey).Result()
	if err != nil {
		t.Fatalf("load hot rank size failed: %v", err)
	}

	if total != domain.HotRankMaxSize {
		t.Fatalf("expected rank trimmed to %d, got %d", domain.HotRankMaxSize, total)
	}

	// 裁剪按分值从低到高，最低的 overflow 个成员应被移除。
	for i := 1; i <= overflow; i++ {
		if _, err := redisClient.ZScore(ctx, constants.VideoHotRankZSetKey, strconv.Itoa(i)).Result(); err == nil {
			t.Fatalf("expected lowest member %d to be trimmed", i)
		}
	}
}

func TestDecayVideoHotScoreSkipsWhenRedisMissing(t *testing.T) {
	// Redis 未配置时只告警跳过，不 panic。
	app.NewHotRankDecayService(NewHotRankStore(nil), zap.NewNop()).Run(context.Background())
}
