package identity

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openUserProfileTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "user-profile.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	statements := []string{
		`CREATE TABLE users (
			id integer PRIMARY KEY, created_at datetime, updated_at datetime,
			username text, email text, password text, avatar text, description text,
 refresh_token_hash text, must_change_password numeric,
			gender integer, birthday datetime, level integer, open_id text,
			register_source text, exp integer, coin_count numeric, status integer,
			current_role_id integer, register_ip text, last_login_ip text,
			register_addr text, last_login_addr text
		)`,
		`CREATE TABLE user_confs (
			user_id integer PRIMARY KEY, like_tags json, banner_id integer,
			banner_url text, open_collect numeric, open_fans numeric,
			open_follow numeric, open_like_video numeric, open_coin_video numeric
		)`,
		`INSERT INTO users (
			id, username, email, password, avatar, description, gender, level,
			open_id, register_source, exp, coin_count, status, current_role_id
		) VALUES (
			1, 'joined-user', 'joined-user@example.com', 'password-hash',
			'avatar.png', 'profile', 1, 3, 'openid', 'github', 120, 4.5, 1, 2
		)`,
		`INSERT INTO user_confs (
			user_id, like_tags, banner_id, banner_url, open_collect, open_fans,
			open_follow, open_like_video, open_coin_video
		) VALUES (
			1, '["Go","DDD"]', 3, 'banner.png', 1, 1, 1, 1, 1
		)`,
		`INSERT INTO users (id, username, email, password) VALUES (
			2, 'no-conf-user', 'no-conf@example.com', 'password-hash'
		)`,
	}

	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("prepare user profile schema: %v", err)
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close sqlite: %v", err)
		}
	})

	return db
}

func TestUserRepository_FindWithConfByIDUsesSingleQuery(t *testing.T) {
	db := openUserProfileTestDB(t)
	repository := NewUserRepository(db, nil)

	var queryCount atomic.Int32
	if err := db.Callback().Query().Before("gorm:query").
		Register("test:count_user_profile_queries", func(*gorm.DB) {
			queryCount.Add(1)
		}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}

	user, conf, err := repository.FindWithConfByID(t.Context(), 1)
	if err != nil {
		t.Fatalf("find user with conf: %v", err)
	}
	if user == nil {
		t.Fatal("user = nil, want joined user")
	}
	if conf == nil {
		t.Fatal("conf = nil, want joined config")
	}
	if user.Username != "joined-user" || user.OpenID != "openid" {
		t.Fatalf("user = %+v, want joined-user with openid", user)
	}
	if conf.BannerID != 3 || conf.BannerURL != "banner.png" {
		t.Fatalf("conf = %+v, want banner ID 3 and URL banner.png", conf)
	}
	if len(conf.LikeTags) != 2 || conf.LikeTags[0] != "Go" || conf.LikeTags[1] != "DDD" {
		t.Fatalf("like tags = %v, want [Go DDD]", conf.LikeTags)
	}
	if got := queryCount.Load(); got != 1 {
		t.Fatalf("query count = %d, want 1", got)
	}
}

func TestUserRepository_FindWithConfByIDAllowsMissingConf(t *testing.T) {
	db := openUserProfileTestDB(t)
	repository := NewUserRepository(db, nil)

	user, conf, err := repository.FindWithConfByID(t.Context(), 2)
	if err != nil {
		t.Fatalf("find user without conf: %v", err)
	}
	if user == nil || user.ID != 2 {
		t.Fatalf("user = %+v, want user 2", user)
	}
	if conf != nil {
		t.Fatalf("conf = %+v, want nil", conf)
	}
}

func TestUserRepository_FindWithConfByIDPropagatesContextError(t *testing.T) {
	db := openUserProfileTestDB(t)
	repository := NewUserRepository(db, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	user, conf, err := repository.FindWithConfByID(ctx, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if user != nil || conf != nil {
		t.Fatalf("user = %+v, conf = %+v, want nil results", user, conf)
	}
}
