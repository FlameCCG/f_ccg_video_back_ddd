package identity

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/identity"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/pkg/constants"
)

type userInfoAccountRepository struct {
	domain.UserAccountRepository
	user              *domain.User
	conf              *domain.UserConf
	err               error
	findByIDCalls     atomic.Int32
	findWithConfCalls atomic.Int32
}

func (r *userInfoAccountRepository) FindByID(context.Context, uint) (*domain.User, error) {
	r.findByIDCalls.Add(1)

	return r.user, r.err
}

func (r *userInfoAccountRepository) FindWithConfByID(
	context.Context,
	uint,
) (*domain.User, *domain.UserConf, error) {
	r.findWithConfCalls.Add(1)

	return r.user, r.conf, r.err
}

type userInfoConfRepository struct {
	domain.UserConfRepository
	findCalls atomic.Int32
	conf      *domain.UserConf
	err       error
}

func (r *userInfoConfRepository) FindByUserID(context.Context, uint) (*domain.UserConf, error) {
	r.findCalls.Add(1)

	return r.conf, r.err
}

type concurrentBannerCatalog struct {
	started chan<- string
	release <-chan struct{}
	calls   atomic.Int32
}

func (c *concurrentBannerCatalog) FindVisible(
	ctx context.Context,
	bannerID uint,
) (*UserBannerItem, error) {
	c.calls.Add(1)
	if c.started != nil {
		c.started <- "banner"
	}
	if c.release != nil {
		select {
		case <-c.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return &UserBannerItem{ID: bannerID, Cover: "joined-banner.png"}, nil
}

func (*concurrentBannerCatalog) ListVisible(
	context.Context,
	[]uint,
) ([]UserBannerItem, error) {
	return []UserBannerItem{}, nil
}

type concurrentRelationQuery struct {
	started chan<- string
	release <-chan struct{}
	calls   atomic.Int32
}

func (q *concurrentRelationQuery) RelationCounts(
	ctx context.Context,
	_ uint,
	_ bool,
) UserRelationCounts {
	q.calls.Add(1)
	if q.started != nil {
		q.started <- "relations"
	}
	if q.release != nil {
		select {
		case <-q.release:
		case <-ctx.Done():
			return UserRelationCounts{}
		}
	}

	return UserRelationCounts{
		FocusCount:   11,
		FansCount:    12,
		DynamicCount: 13,
	}
}

func (*concurrentRelationQuery) DynamicCount(context.Context, uint) int64 {
	return 0
}

func (*concurrentRelationQuery) ActionPrivacy(
	context.Context,
	uint,
) (UserActionPrivacy, error) {
	return UserActionPrivacy{}, nil
}

func TestProfileService_UserInfoLoadsJoinedProfileAndExtrasConcurrently(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	birthday := time.Date(2000, time.March, 4, 0, 0, 0, 0, time.UTC)
	released := false
	t.Cleanup(func() {
		if !released {
			close(release)
		}
	})

	accounts := &userInfoAccountRepository{
		user: &domain.User{
			ID:             7,
			Username:       "joined-user",
			Email:          "joined@example.com",
			Password:       "password-hash",
			Avatar:         "avatar.png",
			Description:    "profile",
			Gender:         2,
			Birthday:       &birthday,
			Level:          4,
			OpenID:         "oauth-id",
			RegisterSource: "github",
			Exp:            120,
			CoinCount:      4.5,
		},
		conf: &domain.UserConf{
			UserID:   7,
			LikeTags: []string{" Go ", "", "DDD"},
			BannerID: 3,
		},
	}
	confs := &userInfoConfRepository{}
	banners := &concurrentBannerCatalog{started: started, release: release}
	relations := &concurrentRelationQuery{started: started, release: release}
	service := NewProfileService(
		infraconfig.NewProvider(nil),
		nil,
		accounts,
		confs,
		banners,
		relations,
		&userInfoRoleQuery{},
		nil,
		nil,
		nil,
	)

	type outcome struct {
		info *UserInfoResult
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		info, err := service.UserInfo(t.Context(), 7)
		done <- outcome{info: info, err: err}
	}()

	seen := make(map[string]bool, 2)
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for len(seen) < 2 {
		select {
		case name := <-started:
			seen[name] = true
		case result := <-done:
			t.Fatalf("UserInfo completed before both extra queries started: %+v", result)
		case <-timer.C:
			t.Fatalf("extra query starts = %v, want banner and relations", seen)
		}
	}

	close(release)
	released = true

	var result outcome
	select {
	case result = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("UserInfo did not complete after releasing extra queries")
	}
	if result.err != nil {
		t.Fatalf("UserInfo: %v", result.err)
	}
	if result.info == nil {
		t.Fatal("info = nil")
	}
	expected := &UserInfoResult{
		ID:             7,
		Username:       "joined-user",
		Email:          "joined@example.com",
		Avatar:         "avatar.png",
		BannerURL:      "joined-banner.png",
		Description:    "profile",
		Gender:         2,
		Birthday:       "2000-03-04",
		Level:          4,
		Exp:            120,
		CoinCount:      4.5,
		FollowCount:    11,
		FansCount:      12,
		DynamicCount:   13,
		RegisterSource: "github",
		LikeTags:       []string{"Go", "DDD"},
		HasPassword:    true,
		IsThirdParty:   true,
	}
	if !reflect.DeepEqual(result.info, expected) {
		t.Fatalf("info = %+v, want %+v", result.info, expected)
	}
	if got := accounts.findWithConfCalls.Load(); got != 1 {
		t.Fatalf("joined profile calls = %d, want 1", got)
	}
	if got := accounts.findByIDCalls.Load(); got != 0 {
		t.Fatalf("separate user calls = %d, want 0", got)
	}
	if got := confs.findCalls.Load(); got != 0 {
		t.Fatalf("separate conf calls = %d, want 0", got)
	}
	if got := banners.calls.Load(); got != 1 {
		t.Fatalf("banner calls = %d, want 1", got)
	}
	if got := relations.calls.Load(); got != 1 {
		t.Fatalf("relation calls = %d, want 1", got)
	}
}

func TestProfileService_UserInfoMapsJoinedQueryErrorsToNotFound(t *testing.T) {
	accounts := &userInfoAccountRepository{err: errors.New("database unavailable")}
	confs := &userInfoConfRepository{}
	banners := &concurrentBannerCatalog{}
	relations := &concurrentRelationQuery{}
	service := NewProfileService(
		infraconfig.NewProvider(nil),
		nil,
		accounts,
		confs,
		banners,
		relations,
		&userInfoRoleQuery{},
		nil,
		nil,
		nil,
	)

	info, err := service.UserInfo(t.Context(), 7)
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("error = %v, want ErrUserNotFound", err)
	}
	if info != nil {
		t.Fatalf("info = %+v, want nil", info)
	}
	if got := banners.calls.Load(); got != 0 {
		t.Fatalf("banner calls = %d, want 0", got)
	}
	if got := relations.calls.Load(); got != 0 {
		t.Fatalf("relation calls = %d, want 0", got)
	}
}

func TestProfileService_UserDetailMapsConfQueryErrorsToNotFound(t *testing.T) {
	accounts := &userInfoAccountRepository{user: &domain.User{ID: 7, Username: "joined-user"}}
	confs := &userInfoConfRepository{err: errors.New("database unavailable")}
	service := NewProfileService(
		infraconfig.NewProvider(nil),
		nil,
		accounts,
		confs,
		&concurrentBannerCatalog{},
		&concurrentRelationQuery{},
		nil,
		nil,
		nil,
		nil,
	)

	detail, err := service.UserDetail(t.Context(), 7)
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("error = %v, want ErrUserNotFound", err)
	}
	if detail != nil {
		t.Fatalf("detail = %+v, want nil", detail)
	}
	if got := confs.findCalls.Load(); got != 1 {
		t.Fatalf("conf calls = %d, want 1", got)
	}
}

// userInfoRoleQuery 为当前用户资料提供明确的角色读模型。
type userInfoRoleQuery struct {
	UserRoleQuery
	items []UserRoleItem
	err   error
}

func (q *userInfoRoleQuery) LoadRoles(context.Context, uint) ([]UserRoleItem, error) {
	return q.items, q.err
}

func TestProfileUserInfoAdministratorIdentity(t *testing.T) {
	for _, admin := range []bool{false, true} {
		roles := &userInfoRoleQuery{}
		if admin {
			roles.items = []UserRoleItem{{Name: constants.AdminUserRole}}
		}
		service := NewProfileService(infraconfig.NewProvider(nil), nil,
			&userInfoAccountRepository{user: &domain.User{ID: 7}}, &userInfoConfRepository{},
			&concurrentBannerCatalog{}, &concurrentRelationQuery{}, roles, nil, nil, nil)
		info, err := service.UserInfo(t.Context(), 7)
		if err != nil {
			t.Fatal(err)
		}
		if info.IsAdmin != admin {
			t.Fatalf("isAdmin=%v, want %v", info.IsAdmin, admin)
		}
		roles.err = errors.New("role query unavailable")
		if info, err := service.UserInfo(t.Context(), 7); err == nil || info != nil {
			t.Fatal("role lookup error must not grant administrator identity")
		}
	}
}
