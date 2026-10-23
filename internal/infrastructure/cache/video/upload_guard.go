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

// refreshUploadGuardScript 只给仍属于当前 token 的锁续租；过期后换主的锁不能被旧持有者延长。
var refreshUploadGuardScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
return 0
`)

// releaseUploadGuardScript 在一次原子操作中比较所有权再删除，避免旧请求误删新持有者的锁。
var releaseUploadGuardScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)

// UploadHashGuard 用按源哈希共享的带租期锁协调上传、投稿验证与垃圾清理。
// 这是协作协议：参与方都须使用同一 key；Redis 锁不覆盖绕过协议的存储写入或删除。
type UploadHashGuard struct {
	redis *redis.Client
}

var _ app.UploadHashGuard = (*UploadHashGuard)(nil)

func NewUploadHashGuard(client *redis.Client) *UploadHashGuard {
	return &UploadHashGuard{redis: client}
}

// Run 等待取得全部哈希锁后执行回调，支持响应请求取消；回调返回后逆序释放。
func (g *UploadHashGuard) Run(ctx context.Context, hashes []string, fn func(context.Context) error) error {
	_, err := g.run(ctx, hashes, true, fn)
	return err
}

// TryRun 不等待占用中的哈希，返回 false 让清理任务本轮跳过，避免与正在投稿的用户竞争。
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
	// 所有参与者按同样顺序取得多把锁，避免不同分P组合以相反顺序持锁等待。
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
	// 续租失败会取消回调 context 并优先返回失锁错误。
	// 回调需及时响应取消；租期锁本身不能强制停止已在执行的数据库或存储操作。
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
