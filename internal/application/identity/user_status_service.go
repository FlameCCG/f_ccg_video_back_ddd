package identity

import (
	"context"
	"strconv"

	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	jwts "f_ccg_video_back_ddd/pkg/jwt"

	"golang.org/x/sync/singleflight"
)

// UserStatusService 用户状态缓存用例服务。
//
// 请求合并通过服务实例内的 singleflight.Group 协调。
type UserStatusService struct {
	cache UserStatusCache
	users identitydomain.UserRepository
	group singleflight.Group
}

// 编译期断言：可作为其它上下文的用户状态读取端口。
var _ UserStatusReader = (*UserStatusService)(nil)

// NewUserStatusService 构造用户状态缓存服务。
func NewUserStatusService(cache UserStatusCache, users identitydomain.UserRepository) *UserStatusService {
	return &UserStatusService{cache: cache, users: users}
}

// CachedStatus 从 Redis cache-aside 获取用户状态；Redis 不可用或未命中时回源 DB。
// 不设进程内 L1，避免 Invalidate 后旧值回填与多实例不一致。
func (s *UserStatusService) CachedStatus(ctx context.Context, userID uint) (enum.UserStatus, error) {
	if userID == 0 {
		return 0, nil
	}

	if status, ok := s.cache.Load(ctx, userID); ok {
		return status, nil
	}

	key := strconv.FormatUint(uint64(userID), 10)

	value, err, _ := s.group.Do(key, func() (any, error) {
		if status, ok := s.cache.Load(ctx, userID); ok {
			return status, nil
		}

		// gen 必须在 load DB 之前捕获，防止封禁后用新 gen 写回旧 Active。
		expectedGen := s.cache.Generation(ctx, userID)

		user, err := s.users.FindStatusByID(ctx, userID)
		if err != nil {
			return enum.UserStatus(0), err
		}

		if user == nil {
			return enum.UserStatus(0), identitydomain.ErrUserRecordNotFound
		}

		s.cache.StoreIfGenerationMatch(ctx, userID, user.Status(), expectedGen)

		return user.Status(), nil
	})
	if err != nil {
		return 0, err
	}

	status, _ := value.(enum.UserStatus)

	return status, nil
}

// Invalidate 删除状态缓存并 bump generation，同时取消本机 singleflight。
func (s *UserStatusService) Invalidate(userID uint) {
	if userID == 0 {
		return
	}

	s.group.Forget(strconv.FormatUint(uint64(userID), 10))
	s.cache.Invalidate(userID)
}

// Refresh 在提交后从最新数据库事实回填，避免解封之后的新封禁被无条件 Active 写穿覆盖。
// Invalidate 作废旧回源，CachedStatus 用读取前的 generation 执行 CAS。
func (s *UserStatusService) Refresh(ctx context.Context, userID uint) error {
	s.Invalidate(userID)
	_, err := s.CachedStatus(ctx, userID)
	return err
}

// ValidateAccessToken checks current DB credential state on every authenticated
// request so password changes revoke tokens across all server instances.
func (s *UserStatusService) ValidateAccessToken(ctx context.Context, token, secret string) (*identitydomain.User, error) {
	claims, err := jwts.ParseAccessToken(token, secret)
	if err != nil {
		return nil, err
	}
	id, err := strconv.ParseUint(claims.UID, 10, strconv.IntSize)
	if err != nil || id == 0 {
		return nil, identitydomain.ErrUserNotLogin
	}
	user, err := s.users.FindByID(ctx, uint(id))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, identitydomain.ErrUserNotFound
	}
	if err := claims.CheckPasswordVersion(secret, user.Password()); err != nil {
		return nil, err
	}
	if err := user.EnsureCanLogin(); err != nil {
		return nil, err
	}
	return user, nil
}
