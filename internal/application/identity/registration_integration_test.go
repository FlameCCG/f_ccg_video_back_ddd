package identity_test

import (
	"context"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/identity"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	persistence "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type registrationRoles struct{ app.UserRoleQuery }

func (registrationRoles) RoleIDByName(context.Context, string) (uint, error) { return 1, nil }

type registrationBinder struct{ calls int }

func (b *registrationBinder) AddRoleForUser(uint, string) error { b.calls++; return nil }
func TestRegistrationCreatesDefaultsInSameTransaction(t *testing.T) {
	t.Parallel()

	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
	for _, failure := range []bool{false, true} {
		name := "success"
		if failure {
			name = "defaults failure rolls back account"
		}

		t.Run(name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "registration.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)

			sqlDB, err := db.DB()
			require.NoError(t, err)

			t.Cleanup(func() { _ = sqlDB.Close() })

			{
				err := db.AutoMigrate(&po.User{}, &po.UserConf{}, &po.FavoriteFolder{}, &po.UserRole{})
				require.NoError(t, err)
			}

			if failure {
				{
					err := db.Exec(`CREATE TRIGGER reject_folder BEFORE INSERT ON favorite_folders BEGIN SELECT RAISE(ABORT,'folder failure'); END`).Error
					require.NoError(t, err)
				}
			}

			provider := infraconfig.NewProvider(&config.Config{Site: &site.Site{DefaultUserBannerID: 29}})
			repo := persistence.NewUserRepository(db, provider)
			binder := &registrationBinder{}
			service := app.NewRegistrationService(gormx.NewTxManager(db), repo, registrationRoles{}, binder, zap.NewNop())

			err = service.EmailRegister(t.Context(), app.EmailRegisterInput{Username: "new-user", Email: "new@example.com", Password: "initial-password"})
			if failure {
				require.Error(t, err, "defaults failure ignored")

				for _, table := range []string{"users", "user_confs", "favorite_folders", "user_roles"} {
					var count int64
					{
						err := db.Table(table).Count(&count).Error
						require.NoError(t, err)
					}

					require.EqualValues(t, 0, count, "partial registration in %s: %d", table, count)
				}

				require.Equal(t, 0, binder.calls, "roles bound before committed account")

				return
			}

			require.NoError(t, err)

			var user po.User
			{
				err := db.First(&user, "username = ?", "new-user").Error
				require.NoError(t, err)
			}

			var conf po.UserConf
			{
				err := db.First(&conf, "user_id = ?", user.ID).Error
				require.NoError(t, err)
			}

			require.EqualValues(t, 29, conf.BannerID, "banner=%d", conf.BannerID)

			var folder po.FavoriteFolder
			{
				err := db.First(&folder, "user_id = ? AND is_default = ?", user.ID, true).Error
				require.NoError(t, err)
			}

			require.Equal(t, 1, binder.calls, "role bindings=%d", binder.calls)
		})
	}
}
