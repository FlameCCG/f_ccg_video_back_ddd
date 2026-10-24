package identity_test

import (
	"context"
	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	persistence "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"path/filepath"
	"testing"
)

type registrationRoles struct{ app.UserRoleQuery }

func (registrationRoles) RoleIDByName(context.Context, string) (uint, error) { return 1, nil }

type registrationBinder struct{ calls int }

func (b *registrationBinder) AddRoleForUser(uint, string) error { b.calls++; return nil }
func TestRegistrationCreatesDefaultsInSameTransaction(t *testing.T) {
	for _, failure := range []bool{false, true} {
		name := "success"
		if failure {
			name = "defaults failure rolls back account"
		}
		t.Run(name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "registration.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			if err := db.AutoMigrate(&po.User{}, &po.UserConf{}, &po.FavoriteFolder{}, &po.UserRole{}); err != nil {
				t.Fatal(err)
			}
			if failure {
				if err := db.Exec(`CREATE TRIGGER reject_folder BEFORE INSERT ON favorite_folders BEGIN SELECT RAISE(ABORT,'folder failure'); END`).Error; err != nil {
					t.Fatal(err)
				}
			}
			provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{DefaultUserBannerID: 29}})
			repo := persistence.NewUserRepository(db, provider)
			binder := &registrationBinder{}
			service := app.NewRegistrationService(gormx.NewTxManager(db), repo, registrationRoles{}, binder, zap.NewNop())
			err = service.EmailRegister(t.Context(), app.EmailRegisterInput{Username: "new-user", Email: "new@example.com", Password: "initial-password"})
			if failure {
				if err == nil {
					t.Fatal("defaults failure ignored")
				}
				for _, table := range []string{"users", "user_confs", "favorite_folders", "user_roles"} {
					var count int64
					if err := db.Table(table).Count(&count).Error; err != nil {
						t.Fatal(err)
					}
					if count != 0 {
						t.Fatalf("partial registration in %s: %d", table, count)
					}
				}
				if binder.calls != 0 {
					t.Fatal("roles bound before committed account")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var user po.User
			if err := db.First(&user, "username = ?", "new-user").Error; err != nil {
				t.Fatal(err)
			}
			var conf po.UserConf
			if err := db.First(&conf, "user_id = ?", user.ID).Error; err != nil {
				t.Fatal(err)
			}
			if conf.BannerID != 29 {
				t.Fatalf("banner=%d", conf.BannerID)
			}
			var folder po.FavoriteFolder
			if err := db.First(&folder, "user_id = ? AND is_default = ?", user.ID, true).Error; err != nil {
				t.Fatal(err)
			}
			if binder.calls != 1 {
				t.Fatalf("role bindings=%d", binder.calls)
			}
		})
	}
}
