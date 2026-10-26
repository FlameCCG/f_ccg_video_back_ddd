package identity

import (
	"context"
	jwts "f_ccg_video_back_ddd/pkg/jwt"
	"strconv"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"

	"golang.org/x/sync/singleflight"
)

// UserStatusService 用户状态缓存用例服务（原 service/user_service/status_cache.go）。
//
// 原实现使用包级 singleflight.Group；新架构把它变为服务实例字段，
// 由 wire 注入单实例，语义不变（同一进程内相同 userID 的回源合并）。
type UserStatusService struct {
	cache domain.UserStatusCache
	users domain.UserRepository
	group singleflight.Group
}

// 编译期断言：可作为其它上下文的用户状态读取端口。
var _ UserStatusReader = (*UserStatusService)(nil)

// NewUserStatusService 构造用户状态缓存服务。
func NewUserStatusService(cache domain.UserStatusCache, users domain.UserRepository) *UserStatusService {
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

		// P1-3: gen 必须在 load DB 之前捕获，防止封禁后用新 gen 写回旧 Active。
		expectedGen := s.cache.Generation(ctx, userID)

		user, err := s.users.FindStatusByID(ctx, userID)
		if err != nil {
			return enum.UserStatus(0), err
		}

		if user == nil {
			return enum.UserStatus(0), domain.ErrUserRecordNotFound
		}

		s.cache.StoreIfGenerationMatch(ctx, userID, user.Status, expectedGen)

		return user.Status, nil
	})
	if err != nil {
		return 0, err
	}

	status, _ := value.(enum.UserStatus)

	return status, nil
}

// Write 权威写穿：封禁/解封成功后直接写入最新状态并 bump generation。
// 比单纯 DEL 更能保证全实例尽快读到 Banned/Active，并作废在途旧回填。
func (s *UserStatusService) Write(userID uint, status enum.UserStatus) {
	if userID == 0 {
		return
	}

	s.group.Forget(strconv.FormatUint(uint64(userID), 10))
	s.cache.Write(userID, status)
}

// Invalidate 删除状态缓存并 bump generation，同时取消本机 singleflight。
func (s *UserStatusService) Invalidate(userID uint) {
	if userID == 0 {
		return
	}

	s.group.Forget(strconv.FormatUint(uint64(userID), 10))
	s.cache.Invalidate(userID)
}

// ValidateAccessToken checks current DB credential state on every authenticated
// request so password changes revoke tokens across all server instances.
func (s *UserStatusService) ValidateAccessToken(ctx context.Context, token, secret string) (*domain.User, error) {
	claims, err := jwts.ParseAccessToken(token, secret)
	if err != nil {
		return nil, err
	}
	id, err := strconv.ParseUint(claims.UID, 10, strconv.IntSize)
	if err != nil || id == 0 {
		return nil, domain.ErrUserNotLogin
	}
	user, err := s.users.FindByID(ctx, uint(id))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, domain.ErrUserNotFound
	}
	if err := claims.CheckPasswordVersion(secret, user.Password); err != nil {
		return nil, err
	}
	if user.IsBanned() {
		return nil, domain.ErrAccountBanned
	}
	return user, nil
}
