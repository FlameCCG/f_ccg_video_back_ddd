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

// StateStore 一次性 OAuth state 存储：Redis 优先，Redis 缺失时回退进程内内存。
//
// 仅在未配置 Redis 句柄时使用内存，Redis 运行故障会直接返回错误。
// 内存 state 不跨实例共享，生成授权链接与处理回调必须落在持有该 state 的实例。
//
// 原实现是 service/login_service 的包级 map + init() 初始化；
// 新架构改为可注入实例，内存兜底状态收敛为结构体字段。
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
//
// Redis GetDel 在同一原子操作中读取并删除，不能替换为分开的 GET/DEL，否则并发回调可能重复通过。
// 内存分支在互斥锁内查询并删除；即使发现已过期，也不恢复该条目。
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
