package identity_test

import (
	"context"
	app "f_ccg_video_back_ddd/internal/application/identity"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	cacheidentity "f_ccg_video_back_ddd/internal/infrastructure/cache/identity"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"testing"
)

type refreshStatusRepo struct {
	identitydomain.UserRepository
	load func() *identitydomain.User
}

func (r refreshStatusRepo) FindStatusByID(context.Context, uint) (*identitydomain.User, error) {
	return r.load(), nil
}

func TestStatusRefreshRejectsReadOvertakenByNewBan(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := cacheidentity.NewUserStatusCache(client, zap.NewNop())
	cache.Write(1, enum.UserStatusBanned)
	read := false
	repo := refreshStatusRepo{load: func() *identitydomain.User {
		read = true
		old := identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 1, Status: enum.UserStatusActive})
		// An administrator publishes a later ban after the old database result was captured.
		cache.Write(1, enum.UserStatusPermanentlyBanned)
		return old
	}}
	if err := app.NewUserStatusService(cache, repo, nil).Refresh(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	status, ok := cache.Load(t.Context(), 1)
	if !read || !ok || status != enum.UserStatusPermanentlyBanned {
		t.Fatalf("stale read replaced newer ban: read=%v cached=%v ok=%v", read, status, ok)
	}
}

func TestStatusRefreshRejectsStaleFillDuringInvalidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		previous  enum.UserStatus
		committed enum.UserStatus
	}{
		{"ban", enum.UserStatusActive, enum.UserStatusBanned},
		{"unban", enum.UserStatusBanned, enum.UserStatusActive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: server.Addr()})
			other := redis.NewClient(&redis.Options{Addr: server.Addr()})
			t.Cleanup(func() { _ = client.Close(); _ = other.Close() })
			cache := cacheidentity.NewUserStatusCache(client, zap.NewNop())
			staleWriter := cacheidentity.NewUserStatusCache(other, zap.NewNop())
			cache.Write(1, tc.previous)
			generation := cache.Generation(t.Context(), 1)
			hook := &statusInvalidationInterleaver{
				commands: other,
				refill: func(ctx context.Context) {
					staleWriter.StoreIfGenerationMatch(ctx, 1, tc.previous, generation)
				},
			}
			client.AddHook(hook)
			reads := 0
			repo := refreshStatusRepo{load: func() *identitydomain.User {
				reads++
				return identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 1, Status: tc.committed})
			}}

			if err := app.NewUserStatusService(cache, repo, nil).Refresh(t.Context(), 1); err != nil {
				t.Fatal(err)
			}
			status, ok := cache.Load(t.Context(), 1)
			if !ok || status != tc.committed || reads != 1 {
				t.Fatalf("cached=%v hit=%v DB reads=%d; want committed=%v and one DB read", status, ok, reads, tc.committed)
			}
			if hook.invalidations != 1 || hook.refills == 0 {
				t.Fatalf("invalidation schedule was not exercised: invalidations=%d refills=%d", hook.invalidations, hook.refills)
			}
			if got := cache.Generation(t.Context(), 1); got != generation+1 {
				t.Fatalf("generation=%d, want %d", got, generation+1)
			}
		})
	}
}

// 普通 pipeline 的命令之间允许另一连接执行旧回源；事务只能在整个事务前后交错。
// 以确定性调度覆盖缓存并发语义，不依赖网络分包或 sleep 的偶然时序。
type statusInvalidationInterleaver struct {
	commands      *redis.Client
	refill        func(context.Context)
	invalidations int
	refills       int
}

func (*statusInvalidationInterleaver) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (*statusInvalidationInterleaver) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return next
}

func (h *statusInvalidationInterleaver) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, commands []redis.Cmder) error {
		invalidates := false
		for _, command := range commands {
			if command.Name() == "del" {
				invalidates = true
				break
			}
		}
		if !invalidates {
			return next(ctx, commands)
		}
		h.invalidations++
		if commands[0].Name() == "multi" {
			h.refill(ctx)
			h.refills++
			if err := next(ctx, commands); err != nil {
				return err
			}
			h.refill(ctx)
			h.refills++
			return nil
		}
		for _, command := range commands {
			if err := h.commands.Process(ctx, command); err != nil {
				return err
			}
			if command.Name() == "del" {
				h.refill(ctx)
				h.refills++
			}
		}
		return nil
	}
}
