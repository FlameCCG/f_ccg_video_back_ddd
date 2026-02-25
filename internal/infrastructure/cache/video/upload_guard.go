package video

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const uploadGuardLease = 2 * time.Minute

var refreshUploadGuardScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
return 0
`)
var releaseUploadGuardScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)

type UploadHashGuard struct {
	redis *redis.Client
}

var _ app.UploadHashGuard = (*UploadHashGuard)(nil)

func NewUploadHashGuard(client *redis.Client) *UploadHashGuard {
	return &UploadHashGuard{redis: client}
}

func (g *UploadHashGuard) Run(ctx context.Context, hashes []string, fn func(context.Context) error) error {
	_, err := g.run(ctx, hashes, true, fn)
	return err
}

func (g *UploadHashGuard) TryRun(ctx context.Context, hash string, fn func(context.Context) error) (bool, error) {
	return g.run(ctx, []string{hash}, false, fn)
}

func (g *UploadHashGuard) run(ctx context.Context, hashes []string, wait bool, fn func(context.Context) error) (bool, error) {
	if g == nil || g.redis == nil {
		return false, errors.New("redis unavailable")
	}
	unique := make(map[string]struct{}, len(hashes))
	for _, hash := range hashes {
		if hash != "" {
			unique[hash] = struct{}{}
		}
	}
	keys := make([]string, 0, len(unique))
	for hash := range unique {
		keys = append(keys, "video:upload:processing:"+hash)
	}
	// A stable order prevents two multipart submissions from waiting on each other.
	sort.Strings(keys)
	if len(keys) == 0 {
		return true, fn(ctx)
	}
	token := uuid.NewString()
	acquired := make([]string, 0, len(keys))
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		for i := len(acquired) - 1; i >= 0; i-- {
			_ = releaseUploadGuardScript.Run(releaseCtx, g.redis, []string{acquired[i]}, token).Err()
		}
	}()
	for _, key := range keys {
		for {
			ok, err := g.redis.SetNX(ctx, key, token, uploadGuardLease).Result()
			if err != nil {
				return false, err
			}
			if ok {
				acquired = append(acquired, key)
				break
			}
			if !wait {
				return false, nil
			}
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			case <-time.After(75 * time.Millisecond):
			}
		}
	}
	guardCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	lost := make(chan error, 1)
	go func() {
		defer close(done)
		ticker := time.NewTicker(uploadGuardLease / 4)
		defer ticker.Stop()
		for {
			select {
			case <-guardCtx.Done():
				return
			case <-ticker.C:
				for _, key := range acquired {
					ok, err := refreshUploadGuardScript.Run(guardCtx, g.redis, []string{key}, token, uploadGuardLease.Milliseconds()).Int()
					if err != nil || ok != 1 {
						if err == nil {
							err = fmt.Errorf("upload lock lost: %s", key)
						}
						lost <- err
						cancel()
						return
					}
				}
			}
		}
	}()
	err := fn(guardCtx)
	cancel()
	<-done
	select {
	case lockErr := <-lost:
		return true, lockErr
	default:
		return true, err
	}
}
