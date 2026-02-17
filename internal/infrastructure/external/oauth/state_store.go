package oauth

import (
	"context"
	"strings"
	"sync"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"
	domain "f_ccg_video_back_ddd/internal/domain/identity"

	"github.com/redis/go-redis/v9"
)

// stateEntry 内存兜底存储中的 state 条目。
type stateEntry struct {
	expiresAt time.Time
}

// StateStore 使用 Redis 保存一次性 OAuth state，未配置 Redis 时使用实例内存回退。
type StateStore struct {
	redis *redis.Client

	mu    sync.Mutex
	items map[string]stateEntry
}

var _ app.OAuthStateStore = (*StateStore)(nil)

// NewStateStore 构造 OAuth state 存储。redisClient 为 nil 时全部走内存兜底。
func NewStateStore(redisClient *redis.Client) *StateStore {
	return &StateStore{
		redis: redisClient,
		items: make(map[string]stateEntry),
	}
}

// Store 保存一次性 OAuth state，TTL 为 domain.OAuthStateTTL。
func (s *StateStore) Store(ctx context.Context, provider, state string) error {
	provider, state, err := domain.NormalizeOAuthState(provider, state)
	if err != nil {
		return err
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	key := domain.OAuthStateKey(provider, state)
	if s.redis != nil {
		return s.redis.Set(ctx, key, "1", domain.OAuthStateTTL).Err()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[key] = stateEntry{expiresAt: time.Now().Add(domain.OAuthStateTTL)}

	return nil
}

// Consume 校验并消费 OAuth state，防止重放。
func (s *StateStore) Consume(ctx context.Context, provider, state string) error {
	provider, state, err := domain.NormalizeOAuthState(provider, state)
	if err != nil {
		return err
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	key := domain.OAuthStateKey(provider, state)
	if s.redis != nil {
		value, err := s.redis.GetDel(ctx, key).Result()
		if err != nil {
			return err
		}

		if strings.TrimSpace(value) == "" {
			return domain.ErrOAuthStateInvalid
		}

		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.items[key]
	if !ok {
		return domain.ErrOAuthStateInvalid
	}

	delete(s.items, key)

	if time.Now().After(entry.expiresAt) {
		return domain.ErrOAuthStateInvalid
	}

	return nil
}
