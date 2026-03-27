package identity

import (
	"context"
	"sync"
	"time"
)

type securitySessionEntry struct {
	digest    string
	expiresAt time.Time
	previous  map[string]time.Time
}

// 应用测试使用受锁保护的内存会话；真实 Redis 的原子性与 TTL 在适配器测试中验证。
type securitySessionStore struct {
	mu       sync.Mutex
	sessions map[uint]map[string]securitySessionEntry
	err      error
	now      func() time.Time
}

func newSecuritySessionStore() *securitySessionStore {
	return &securitySessionStore{sessions: make(map[uint]map[string]securitySessionEntry), now: time.Now}
}

func (s *securitySessionStore) Create(_ context.Context, userID uint, id, digest string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	if s.sessions[userID] == nil {
		s.sessions[userID] = make(map[string]securitySessionEntry)
	}
	s.sessions[userID][id] = securitySessionEntry{digest: digest, expiresAt: expiresAt, previous: make(map[string]time.Time)}
	return nil
}

func (s *securitySessionStore) Exists(_ context.Context, userID uint, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return false, s.err
	}
	entry, ok := s.sessions[userID][id]
	return ok && entry.expiresAt.After(s.now()), nil
}

func (s *securitySessionStore) Rotate(_ context.Context, userID uint, id, oldHash, newHash string, expiresAt time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return false, s.err
	}
	entry, ok := s.sessions[userID][id]
	if !ok || !entry.expiresAt.After(s.now()) {
		return false, nil
	}
	if entry.digest == newHash || entry.previous[newHash].After(s.now()) {
		return true, nil
	}
	if entry.digest != oldHash && !entry.previous[oldHash].After(s.now()) {
		delete(s.sessions[userID], id)
		return false, ErrRefreshTokenReused
	}
	if _, consumed := entry.previous[entry.digest]; !consumed {
		entry.previous[entry.digest] = s.now().Add(30 * time.Second)
	}
	entry.digest, entry.expiresAt = newHash, expiresAt
	s.sessions[userID][id] = entry
	return true, nil
}

func (s *securitySessionStore) SnapshotUser(_ context.Context, userID uint) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	var ids []string
	for id := range s.sessions[userID] {
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *securitySessionStore) RevokeSessions(_ context.Context, userID uint, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	for _, id := range ids {
		delete(s.sessions[userID], id)
	}
	return nil
}
