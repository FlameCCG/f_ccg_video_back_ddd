package identity

import (
	"context"
	"errors"
	"strconv"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"

	"github.com/redis/go-redis/v9"
)

// 创作中心指标 Redis key 前缀（属于外部契约，与原实现完全一致）。
const (
	creatorAnalyticsFansKeyPrefix      = "creator:analytics:fans:"
	creatorAnalyticsViewsKeyPrefix     = "creator:analytics:views:"
	creatorAnalyticsCommentsKeyPrefix  = "creator:analytics:comments:"
	creatorAnalyticsCoinsKeyPrefix     = "creator:analytics:coins:"
	creatorAnalyticsDanmuKeyPrefix     = "creator:analytics:danmu:"
	creatorAnalyticsFavoritesKeyPrefix = "creator:analytics:favorites:"
)

// errRedisNotInitialized Redis 未初始化（文案与原实现一致）。
var errRedisNotInitialized = errors.New("redis 未初始化")

// errCreatorIDRequired creatorId 缺失（文案与原实现一致）。
var errCreatorIDRequired = errors.New("creatorId 不能为空")

// CreatorMetricStore 创作中心指标存储的 Redis 实现
// （原 service/user_service/creator_analytics_service.go 的 incrCreatorMetric / HMGet）。
type CreatorMetricStore struct {
	redis *redis.Client
}

var _ app.CreatorMetricStore = (*CreatorMetricStore)(nil)

// NewCreatorMetricStore 构造创作中心指标存储适配器。
func NewCreatorMetricStore(redisClient *redis.Client) *CreatorMetricStore {
	return &CreatorMetricStore{redis: redisClient}
}

// Available 报告 Redis 是否可用。
func (s *CreatorMetricStore) Available() error {
	if s.redis == nil {
		return errRedisNotInitialized
	}

	return nil
}

// Incr 累加指定指标在当天的计数（Redis Hash，field = yyyyMMdd）。
func (s *CreatorMetricStore) Incr(ctx context.Context, metric string, creatorID uint, delta int64) error {
	if s.redis == nil {
		return errRedisNotInitialized
	}

	if creatorID == 0 {
		return errCreatorIDRequired
	}

	if delta == 0 {
		return nil
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	prefix, err := metricKeyPrefix(metric)
	if err != nil {
		return err
	}

	day := time.Now().Format("20060102")
	key := prefix + strconv.FormatUint(uint64(creatorID), 10)

	return s.redis.HIncrBy(ctx, key, day, delta).Err()
}

// Series 按天字段批量读取指标值，缺失位置补 0。
func (s *CreatorMetricStore) Series(
	ctx context.Context,
	metric string,
	creatorID uint,
	dayFields []string,
) ([]int64, error) {
	if s.redis == nil {
		return nil, errRedisNotInitialized
	}

	prefix, err := metricKeyPrefix(metric)
	if err != nil {
		return nil, err
	}

	key := prefix + strconv.FormatUint(uint64(creatorID), 10)

	valuesRaw, err := s.redis.HMGet(ctx, key, dayFields...).Result()
	if err != nil {
		return nil, err
	}

	return parseHMGetInt64(valuesRaw, len(dayFields)), nil
}

// metricKeyPrefix 把指标名映射为 Redis key 前缀。
func metricKeyPrefix(metric string) (string, error) {
	switch metric {
	case app.CreatorAnalyticsTypeFans:
		return creatorAnalyticsFansKeyPrefix, nil
	case "", app.CreatorAnalyticsTypeViews:
		return creatorAnalyticsViewsKeyPrefix, nil
	case app.CreatorAnalyticsTypeComments:
		return creatorAnalyticsCommentsKeyPrefix, nil
	case app.CreatorAnalyticsTypeCoins:
		return creatorAnalyticsCoinsKeyPrefix, nil
	case app.CreatorAnalyticsTypeDanmu:
		return creatorAnalyticsDanmuKeyPrefix, nil
	case app.CreatorAnalyticsTypeFavorites:
		return creatorAnalyticsFavoritesKeyPrefix, nil
	default:
		return "", app.ErrCreatorMetricTypeInvalid
	}
}

// parseHMGetInt64 把 HMGET 结果解析为 int64 序列，无法解析的补 0。
func parseHMGetInt64(values []any, size int) []int64 {
	out := make([]int64, 0, size)

	for i := range size {
		if i >= len(values) || values[i] == nil {
			out = append(out, 0)

			continue
		}

		switch v := values[i].(type) {
		case int64:
			out = append(out, v)
		case string:
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				out = append(out, n)
			} else {
				out = append(out, 0)
			}
		case []byte:
			if n, err := strconv.ParseInt(string(v), 10, 64); err == nil {
				out = append(out, n)
			} else {
				out = append(out, 0)
			}
		default:
			out = append(out, 0)
		}
	}

	return out
}
