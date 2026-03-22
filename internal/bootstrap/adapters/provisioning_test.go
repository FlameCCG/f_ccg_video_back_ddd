package adapters

import (
	"errors"
	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	provisioningapp "f_ccg_video_back_ddd/internal/application/provisioning"
	rbacapp "f_ccg_video_back_ddd/internal/application/rbac"
	videoapp "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	identityrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	rbacrepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/rbac"
	videorepo "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"
	"f_ccg_video_back_ddd/pkg/encrypt"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"path/filepath"
	"testing"
)

type systemRoleBinder struct{ calls int }

func (b *systemRoleBinder) BindUserRole(uint, string) error { b.calls++; return nil }

func systemProvisioningDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "system.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`CREATE TABLE users (id integer PRIMARY KEY AUTOINCREMENT, created_at datetime, updated_at datetime,
 username text, email text, password text, avatar text, description text, refresh_token_hash text, must_change_password numeric,
 gender integer, birthday datetime, level integer, open_id text, register_source text, exp integer, coin_count numeric, status integer,
 current_role_id integer, register_ip text, last_login_ip text, register_addr text, last_login_addr text)`,
		`CREATE TABLE roles (id integer PRIMARY KEY AUTOINCREMENT, created_at datetime, updated_at datetime, name text UNIQUE, "desc" text)`,
		`CREATE TABLE user_roles (user_id integer, role_id integer, PRIMARY KEY(user_id,role_id))`,
		`CREATE TABLE user_confs (user_id integer PRIMARY KEY, like_tags json, banner_id integer, banner_url text,
 open_collect numeric, open_fans numeric, open_follow numeric, open_like_video numeric, open_coin_video numeric)`,
		`CREATE TABLE favorite_folders (id integer PRIMARY KEY AUTOINCREMENT, created_at datetime, updated_at datetime, user_id integer, name text, is_default numeric)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestSystemProvisioningSharesTransactionAcrossContexts(t *testing.T) {
	for _, failBindings := range []bool{false, true} {
		name := "create and reset"
		if failBindings {
			name = "binding failure rolls back all contexts"
		}
		t.Run(name, func(t *testing.T) {
			db := systemProvisioningDB(t)
			if failBindings {
				if err := db.Callback().Create().Before("gorm:create").Register("test:fail_role_binding", func(q *gorm.DB) {
					if q.Statement.Table == "user_roles" {
						q.AddError(errors.New("binding failed"))
					}
				}); err != nil {
					t.Fatal(err)
				}
			}
			accounts := identityapp.NewSystemAccountService(identityrepo.NewUserRepository(db, nil), videoapp.NewDefaultFolderService(videorepo.NewFolderRepository(db)))
			roles := rbacapp.NewSystemRoleService(rbacrepo.NewRoleRepository(db), rbacrepo.NewUserRoleRepository(db), zap.NewNop())
			binder := &systemRoleBinder{}
			service := provisioningapp.NewSuperUserService(NewSystemAccounts(accounts, roles), binder, gormx.NewTxManager(db))
			result, err := service.Provision(t.Context(), []string{"admin", "first-password"})
			if failBindings {
				if err == nil || binder.calls != 0 {
					t.Fatalf("err=%v binder calls=%d", err, binder.calls)
				}
				for _, table := range []string{"users", "roles", "user_roles", "user_confs", "favorite_folders"} {
					var count int64
					if err := db.Table(table).Count(&count).Error; err != nil || count != 0 {
						t.Fatalf("%s leaked rows=%d err=%v", table, count, err)
					}
				}
				return
			}
			if err != nil || !result.UserCreated || !result.RoleCreated || binder.calls != 1 {
				t.Fatalf("result=%+v err=%v binder=%d", result, err, binder.calls)
			}
			if err := db.Exec("UPDATE users SET coin_count=12.3,status=2,avatar='preserved',refresh_token_hash='session' WHERE id=?", result.UserID).Error; err != nil {
				t.Fatal(err)
			}
			result, err = service.Provision(t.Context(), []string{"admin", "next-password"})
			if err != nil || result.UserCreated || result.RoleCreated || binder.calls != 2 {
				t.Fatalf("replay result=%+v err=%v", result, err)
			}
			var user po.User
			if err := db.First(&user, result.UserID).Error; err != nil {
				t.Fatal(err)
			}
			if !encrypt.ValidatePasswordHash("next-password", user.Password) || !user.MustChangePassword || user.RefreshTokenHash != "" || user.Level != 6 || user.Exp != 20000 || user.CurrentRoleID == 0 {
				t.Fatalf("incorrect reset: %+v", user)
			}
			if user.CoinCount != 12.3 || user.Status != 2 || user.Avatar != "preserved" {
				t.Fatalf("reset overwrote unrelated state: %+v", user)
			}
			for _, table := range []string{"users", "roles", "user_roles", "user_confs", "favorite_folders"} {
				var count int64
				if err := db.Table(table).Count(&count).Error; err != nil || count != 1 {
					t.Fatalf("%s replay rows=%d err=%v", table, count, err)
				}
			}
		})
	}
}
