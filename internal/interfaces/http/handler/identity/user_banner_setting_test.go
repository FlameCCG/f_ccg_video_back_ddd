package identity

import (
	"context"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"f_ccg_video_back_ddd/pkg/res"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/identity"

	domain "f_ccg_video_back_ddd/internal/domain/identity"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	contentopspersistence "f_ccg_video_back_ddd/internal/infrastructure/persistence/contentops"

	persistence "f_ccg_video_back_ddd/internal/infrastructure/persistence/identity"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// bannerFixture 汇总横幅相关用例所需的处理器与依赖。
type bannerFixture struct {
	handler      *Handler
	adminHandler *AdminHandler
	provider     *infraconfig.Provider
	db           *gorm.DB
	users        *persistence.UserRepository
}

// newBannerFixture 装配用户横幅用例（注入式：配置由 Provider 持有，配置文件路径注入写回器）。
func newBannerFixture(t *testing.T, cfg *config.Config, configPath string) *bannerFixture {
	t.Helper()

	db := openTestDB(t, &po.User{}, &po.UserConf{}, &po.FavoriteFolder{}, &po.Banner{})
	provider := infraconfig.NewProvider(cfg)

	users := persistence.NewUserRepository(db, provider)
	confs := persistence.NewUserConfRepository(db)
	banners := persistence.NewUserBannerQuery(db)
	writer := contentopspersistence.NewConfigFileWriter(configPath)

	bannerSvc := app.NewUserBannerService(provider, confs, banners, writer, nil, zap.NewNop())
	confSvc := app.NewUserConfService(provider, confs, confs, banners)

	handler := NewHandler(nil, nil, confSvc, bannerSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	adminHandler := NewAdminHandler(nil, nil, nil, nil, bannerSvc, nil, nil, nil)

	return &bannerFixture{
		handler:      handler,
		adminHandler: adminHandler,
		provider:     provider,
		db:           db,
		users:        users,
	}
}

// createUser 走仓储创建路径落库用户，触发 po.User 的 AfterCreate 钩子
// （写 user_confs 与默认收藏夹），用于验证默认横幅 ID 经 ctx 注入。
func (f *bannerFixture) createUser(t *testing.T, userID uint, username, email string) {
	t.Helper()

	user := &domain.User{
		ID:       userID,
		Username: username,
		Email:    email,
	}

	// 与生产路径一致（gormx.TxManager.Do）：先 WithDefaultBanner 包装 ctx，
	// 再 WithContext(ctx).Transaction，这样 AfterCreate 钩子能从 tx.Statement.Context 读到横幅 ID。
	ctx := f.users.WithDefaultBanner(context.Background())
	{
		err := f.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			txCtx := gormx.WithTx(ctx, tx)
			if err := f.users.Create(txCtx, user, 0); err != nil {
				return err
			}

			return f.users.InitializeDefaults(txCtx, user.ID)
		})
		require.NoError(t, err, "create user %d failed: %v", userID, err)
	}
}

// userConf 读取用户配置行。
func (f *bannerFixture) userConf(t *testing.T, userID uint) po.UserConf {
	t.Helper()

	var conf po.UserConf
	{
		err := f.db.First(&conf, "user_id = ?", userID).Error
		require.NoError(t, err, "query user conf %d failed: %v", userID, err)
	}

	return conf
}

// writeTempConfig 写入初始 YAML 配置文件并返回路径。
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	{
		err := os.WriteFile(configPath, []byte(content), 0o600)
		require.NoError(t, err, "write temp config failed: %v", err)
	}

	return configPath
}

// TestUserCreateSetsDefaultBannerID 验证注册默认横幅 ID 经 ctx 注入到 AfterCreate 钩子
// （原 TestUserAfterCreateSetsDefaultBannerID 依赖 global.Config()，见 decisions.md D-006）。
func TestUserCreateSetsDefaultBannerID(t *testing.T) {
	t.Parallel()

	fixture := newBannerFixture(t, &config.Config{
		Site: &site.Site{
			DefaultUserBannerID: 29,
		},
	}, writeTempConfig(t, "site:\n  defaultuserbannerid: 29\n"))

	fixture.createUser(t, 1, "u1", "u1@example.com")

	{
		conf := fixture.userConf(t, 1)
		require.EqualValues(t, 29, conf.BannerID, "expected default banner id 29, got %d", conf.BannerID)
	}
}

// TestUserCreateIgnoresDefaultBannerIDList 验证默认横幅列表不参与新用户横幅选择
// （原 TestUserAfterCreateIgnoresDefaultBannerIDList）。
func TestUserCreateIgnoresDefaultBannerIDList(t *testing.T) {
	t.Parallel()

	fixture := newBannerFixture(t, &config.Config{
		Site: &site.Site{
			DefaultUserBannerID:  29,
			DefaultUserBannerIDs: []uint{31, 29},
		},
	}, writeTempConfig(t, "site:\n  defaultuserbannerid: 29\n"))

	fixture.createUser(t, 1, "u1", "u1@example.com")

	{
		conf := fixture.userConf(t, 1)
		require.EqualValues(t, 29, conf.BannerID, "expected default banner id 29, got %d", conf.BannerID)
	}
}

func TestAdminSetUserBannerView(t *testing.T) {
	t.Parallel()

	configPath := writeTempConfig(t, "site:\n  defaultuserbannerid: 19\n")
	fixture := newBannerFixture(t, &config.Config{
		Site: &site.Site{
			DefaultUserBannerID:  19,
			DefaultUserBannerIDs: nil,
		},
	}, configPath)

	fixture.createUser(t, 1, "u1", "u1@example.com")
	fixture.createUser(t, 2, "u2", "u2@example.com")
	fixture.createUser(t, 3, "u3", "u3@example.com")

	{
		err := fixture.db.Model(&po.UserConf{}).
			Where("user_id = ?", 2).
			Update("banner_url", testCustomBannerURL).Error
		require.NoError(t, err, "seed user banner url failed: %v", err)
	}

	userBannerID := seedVisibleUserBanner(t, fixture.db, "https://cdn.example.com/banner-user.jpg")

	otherUserBannerID := seedVisibleUserBanner(t, fixture.db, "https://cdn.example.com/banner-user-2.jpg")

	homeBannerID := seedBanner(t, fixture.db, &po.Banner{
		Cover: "https://cdn.example.com/banner-home.jpg",
		Show:  true,
		Type:  enum.BannerTypeHome,
	})

	{
		err := fixture.db.Model(&po.UserConf{}).
			Where("user_id = ?", 3).
			Update("banner_id", otherUserBannerID).Error
		require.NoError(t, err, "seed user3 banner id failed: %v", err)
	}

	//nolint:paralleltest // Cases share a mutable parent configuration or database fixture.
	t.Run("success", func(t *testing.T) {
		recorder, ctx := newTestContext(http.MethodPut, "/v1/admin/user/banner")
		ctx.Set("request", AdminSetUserBannerReq{BannerID: userBannerID})

		fixture.adminHandler.AdminSetUserBannerView(ctx)

		require.Equal(t, http.StatusOK, recorder.Code, "unexpected status code: %d", recorder.Code)

		response := decodeResponse(t, recorder)
		require.EqualValues(t, res.SuccessCode, response.Code, "unexpected response code: %d (%s)", response.Code, response.Msg)

		{
			got := fixture.userConf(t, 1).BannerID
			require.Equal(t, got, userBannerID, "expected banner id %d, got %d", userBannerID, got)
		}

		user2Conf := fixture.userConf(t, 2)
		require.Equal(t, user2Conf.BannerID, userBannerID, "expected user2 banner id %d, got %d", userBannerID, user2Conf.BannerID)

		require.Equal(t, testCustomBannerURL, user2Conf.BannerURL, "expected custom banner url preserved, got %q", user2Conf.BannerURL)

		{
			got := fixture.userConf(t, 3).BannerID
			require.Equal(t, got, otherUserBannerID, "expected user3 banner id stay %d, got %d", otherUserBannerID, got)
		}

		cfg := fixture.provider.Get()
		require.NotNil(t, cfg.Site, "expected default banner id %d, got %+v", userBannerID, cfg.Site)
		require.Equal(t, cfg.Site.DefaultUserBannerID, userBannerID, "expected default banner id %d, got %+v", userBannerID, cfg.Site)

		require.Len(t, cfg.Site.DefaultUserBannerIDs, 1, "expected old default banner 19 added to default list, got %+v", cfg.Site.DefaultUserBannerIDs)
		require.EqualValues(t, 19, cfg.Site.DefaultUserBannerIDs[0], "expected old default banner 19 added to default list, got %+v", cfg.Site.DefaultUserBannerIDs)

		content, err := os.ReadFile(configPath) // #nosec G304 -- Reads a fixture created inside this test's private temporary directory.
		require.NoError(t, err, "read temp config failed: %v", err)

		expectedLine := "defaultuserbannerid: " + strconv.FormatUint(uint64(userBannerID), 10)
		require.Contains(t, string(content), expectedLine, "expected config file updated, got: %s", string(content))
	})

	//nolint:paralleltest // Cases share a mutable parent configuration or database fixture.
	t.Run("reject non-user banner", func(t *testing.T) {
		recorder, ctx := newTestContext(http.MethodPut, "/v1/admin/user/banner")
		ctx.Set("request", AdminSetUserBannerReq{BannerID: homeBannerID})

		fixture.adminHandler.AdminSetUserBannerView(ctx)

		response := decodeResponse(t, recorder)
		require.EqualValues(t, res.FailServiceCode, response.Code, "expected fail response code, got %d", response.Code)

		require.Equal(t, "用户主页横幅不存在", response.Msg, "unexpected response msg: %s", response.Msg)
	})

	//nolint:paralleltest // Cases share a mutable parent configuration or database fixture.
	t.Run("reject setting already default banner", func(t *testing.T) {
		currentDefault := domain.ResolveDefaultUserBannerID(fixture.provider.Get().Site.DefaultUserBannerID)

		// 保证该 ID 存在且是可选的用户横幅，从而只剩「已是注册默认横幅」这一失败原因。
		{
			err := fixture.db.Save(&po.Banner{
				Model: po.Model{ID: currentDefault},
				Cover: "https://cdn.example.com/banner-current.jpg",
				Show:  true,
				Type:  enum.BannerTypeUser,
			}).Error
			require.NoError(t, err, "save current default banner failed: %v", err)
		}

		recorder, ctx := newTestContext(http.MethodPut, "/v1/admin/user/banner")
		ctx.Set("request", AdminSetUserBannerReq{BannerID: currentDefault})

		fixture.adminHandler.AdminSetUserBannerView(ctx)

		response := decodeResponse(t, recorder)
		require.EqualValues(t, res.FailServiceCode, response.Code, "expected fail response code, got %d", response.Code)

		require.Equal(t, "该横幅已是注册默认横幅", response.Msg, "unexpected response msg: %s", response.Msg)
	})
}

func TestListDefaultUserBannersView(t *testing.T) {
	t.Parallel()

	fixture := newBannerFixture(t, &config.Config{}, writeTempConfig(t, "site:\n"))

	firstBannerID := seedVisibleUserBanner(t, fixture.db, "https://cdn.example.com/banner-user-1.jpg")

	secondBannerID := seedVisibleUserBanner(t, fixture.db, "https://cdn.example.com/banner-user-2.jpg")

	hiddenBannerID := seedVisibleUserBanner(t, fixture.db, "https://cdn.example.com/banner-user-3.jpg")

	{
		err := fixture.db.Model(&po.Banner{}).Where("id = ?", hiddenBannerID).
			Update("show", false).Error
		require.NoError(t, err, "hide user banner failed: %v", err)
	}

	fixture.provider.Set(&config.Config{
		Site: &site.Site{
			DefaultUserBannerID:  firstBannerID,
			DefaultUserBannerIDs: []uint{secondBannerID, hiddenBannerID},
		},
	})

	seedUserConf(t, fixture.db, po.UserConf{UserID: 7, BannerID: firstBannerID})

	recorder, ctx := newTestContext(http.MethodGet, "/v1/common/user/banner/defaults")
	ctx.Set("uid", "7")

	fixture.handler.ListDefaultUserBannersView(ctx)

	require.Equal(t, http.StatusOK, recorder.Code, "unexpected status code: %d", recorder.Code)

	response := decodeResponse(t, recorder)
	require.EqualValues(t, res.SuccessCode, response.Code, "unexpected response code: %d (%s)", response.Code, response.Msg)

	var resp DefaultUserBannerListRes
	decodeResponseData(t, response, &resp)

	require.Len(t, resp.BannerIDs, 3, "expected 3 configured banner ids, got %d", len(resp.BannerIDs))

	require.Equal(t, resp.BannerIDs[0], secondBannerID, "unexpected banner ids: %+v", resp.BannerIDs)
	require.Equal(t, resp.BannerIDs[1], hiddenBannerID, "unexpected banner ids: %+v", resp.BannerIDs)
	require.Equal(t, resp.BannerIDs[2], firstBannerID, "unexpected banner ids: %+v", resp.BannerIDs)

	require.Equal(t, resp.CurrentBannerID, firstBannerID, "expected current banner id %d, got %d", firstBannerID, resp.CurrentBannerID)

	require.Equal(t, "https://cdn.example.com/banner-user-1.jpg", resp.CurrentBannerImageURL, "unexpected current banner image url: %q", resp.CurrentBannerImageURL)

	require.Len(t, resp.List, 2, "expected 2 visible banners, got %d", len(resp.List))

	require.Equal(t, resp.List[0].ID, secondBannerID, "unexpected banner order: %+v", resp.List)
	require.Equal(t, resp.List[1].ID, firstBannerID, "unexpected banner order: %+v", resp.List)

	t.Run("custom banner returns zero current banner id", func(t *testing.T) {
		{
			err := fixture.db.Model(&po.UserConf{}).
				Where("user_id = ?", 7).
				Updates(map[string]any{
					"banner_url": testCustomBannerURL,
					"banner_id":  firstBannerID,
				}).Error
			require.NoError(t, err, "update custom banner failed: %v", err)
		}

		customRecorder, customCtx := newTestContext(http.MethodGet, "/v1/common/user/banner/defaults")
		customCtx.Set("uid", "7")

		fixture.handler.ListDefaultUserBannersView(customCtx)

		var customResp DefaultUserBannerListRes
		decodeResponseData(t, decodeResponse(t, customRecorder), &customResp)

		require.EqualValues(t, 0, customResp.CurrentBannerID, "expected custom banner current id 0, got %d", customResp.CurrentBannerID)

		require.Equal(t, testCustomBannerURL, customResp.CurrentBannerImageURL, "unexpected custom banner image url: %q", customResp.CurrentBannerImageURL)
	})
}

func TestAdminUpdateUserBannerListView(t *testing.T) {
	t.Parallel()

	fixture := newBannerFixture(t, &config.Config{}, writeTempConfig(t, "site:\n"))

	firstBannerID := seedVisibleUserBanner(t, fixture.db, "https://cdn.example.com/banner-user-1.jpg")

	secondBannerID := seedVisibleUserBanner(t, fixture.db, "https://cdn.example.com/banner-user-2.jpg")

	thirdBannerID := seedVisibleUserBanner(t, fixture.db, "https://cdn.example.com/banner-user-3.jpg")

	homeBannerID := seedBanner(t, fixture.db, &po.Banner{
		Cover: "https://cdn.example.com/banner-home.jpg",
		Show:  true,
		Type:  enum.BannerTypeHome,
	})

	configPath := writeTempConfig(t,
		"site:\n  defaultuserbannerid: "+strconv.FormatUint(uint64(firstBannerID), 10)+"\n")

	// 配置文件路径变了，重建一套依赖指向新文件。
	fixture = rebindBannerFixture(t, fixture, configPath, &config.Config{
		Site: &site.Site{DefaultUserBannerID: firstBannerID},
	})

	//nolint:paralleltest // Cases share a mutable parent configuration or database fixture.
	t.Run("add", func(t *testing.T) {
		checkAddDefaultUserBanners(t, fixture, firstBannerID, secondBannerID, thirdBannerID, configPath)
	})

	//nolint:paralleltest // Cases share a mutable parent configuration or database fixture.
	t.Run("reject adding duplicate banners to defaults list", func(t *testing.T) { checkDuplicateDefaultUserBanner(t, fixture, secondBannerID) })

	//nolint:paralleltest // Cases share a mutable parent configuration or database fixture.
	t.Run("delete from list should not affect register default", func(t *testing.T) {
		checkRemoveDefaultUserBanner(t, fixture, firstBannerID, secondBannerID, thirdBannerID)
	})

	//nolint:paralleltest // Cases share a mutable parent configuration or database fixture.
	t.Run("reject touching register default banner", func(t *testing.T) { checkRegisterDefaultBannerProtected(t, fixture, firstBannerID) })

	//nolint:paralleltest // Cases share a mutable parent configuration or database fixture.
	t.Run("reject non-user banner", func(t *testing.T) { checkRejectHomeBannerFromUserDefaults(t, fixture, homeBannerID) })
}

// rebindBannerFixture 复用已建好的库，但把配置快照与配置文件路径换成新的。
func rebindBannerFixture(
	t *testing.T,
	old *bannerFixture,
	configPath string,
	cfg *config.Config,
) *bannerFixture {
	t.Helper()

	provider := infraconfig.NewProvider(cfg)
	confs := persistence.NewUserConfRepository(old.db)
	banners := persistence.NewUserBannerQuery(old.db)
	writer := contentopspersistence.NewConfigFileWriter(configPath)

	bannerSvc := app.NewUserBannerService(provider, confs, banners, writer, nil, zap.NewNop())

	return &bannerFixture{
		handler:      NewHandler(nil, nil, nil, bannerSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop()),
		adminHandler: NewAdminHandler(nil, nil, nil, nil, bannerSvc, nil, nil, nil),
		provider:     provider,
		db:           old.db,
		users:        persistence.NewUserRepository(old.db, provider),
	}
}

func checkAddDefaultUserBanners(t *testing.T, fixture *bannerFixture, firstBannerID, secondBannerID, thirdBannerID uint, configPath string) {
	t.Helper()

	recorder, ctx := newTestContext(http.MethodPut, "/v1/admin/user/banner/defaults")
	ctx.Set("request", AdminUpdateUserBannerListReq{
		BannerIDs: []uint{secondBannerID, thirdBannerID},
		Action:    domain.UserBannerActionAdd,
	})

	fixture.adminHandler.AdminUpdateUserBannerListView(ctx)

	require.Equal(t, http.StatusOK, recorder.Code, "unexpected status code: %d", recorder.Code)

	response := decodeResponse(t, recorder)
	require.EqualValues(t, res.SuccessCode, response.Code, "unexpected response code: %d (%s)", response.Code, response.Msg)

	cfgSite := fixture.provider.Get().Site
	require.Len(t, cfgSite.DefaultUserBannerIDs, 2, "expected 2 banner ids, got %+v", cfgSite.DefaultUserBannerIDs)

	require.Equal(t, cfgSite.DefaultUserBannerIDs[0], secondBannerID, "unexpected banner ids: %+v", cfgSite.DefaultUserBannerIDs)
	require.Equal(t, cfgSite.DefaultUserBannerIDs[1], thirdBannerID, "unexpected banner ids: %+v", cfgSite.DefaultUserBannerIDs)

	require.Equal(t, cfgSite.DefaultUserBannerID, firstBannerID, "expected register default banner id stay %d, got %d", firstBannerID, cfgSite.DefaultUserBannerID)

	content, err := os.ReadFile(configPath) // #nosec G304 -- Reads a fixture created inside this test's private temporary directory.
	require.NoError(t, err, "read temp config failed: %v", err)

	require.Contains(t, string(content), "defaultuserbannerids:", "expected config file updated with banner list, got: %s", string(content))
}

func checkDuplicateDefaultUserBanner(t *testing.T, fixture *bannerFixture, secondBannerID uint) {
	t.Helper()

	recorder, ctx := newTestContext(http.MethodPut, "/v1/admin/user/banner/defaults")
	ctx.Set("request", AdminUpdateUserBannerListReq{
		BannerIDs: []uint{secondBannerID},
		Action:    domain.UserBannerActionAdd,
	})

	fixture.adminHandler.AdminUpdateUserBannerListView(ctx)

	response := decodeResponse(t, recorder)
	require.EqualValues(t, res.FailServiceCode, response.Code, "expected fail response code, got %d", response.Code)

	require.Equal(t, "所选横幅已存在于系统默认列表中", response.Msg, "unexpected response msg: %s", response.Msg)
}

func checkRemoveDefaultUserBanner(t *testing.T, fixture *bannerFixture, firstBannerID, secondBannerID, thirdBannerID uint) {
	t.Helper()

	recorder, ctx := newTestContext(http.MethodPut, "/v1/admin/user/banner/defaults")
	ctx.Set("request", AdminUpdateUserBannerListReq{
		BannerIDs: []uint{thirdBannerID},
		Action:    domain.UserBannerActionDelete,
	})

	fixture.adminHandler.AdminUpdateUserBannerListView(ctx)

	response := decodeResponse(t, recorder)
	require.EqualValues(t, res.SuccessCode, response.Code, "unexpected response code: %d (%s)", response.Code, response.Msg)

	cfgSite := fixture.provider.Get().Site
	require.Len(t, cfgSite.DefaultUserBannerIDs, 1, "unexpected banner ids after delete: %+v", cfgSite.DefaultUserBannerIDs)
	require.Equal(t, cfgSite.DefaultUserBannerIDs[0], secondBannerID, "unexpected banner ids after delete: %+v", cfgSite.DefaultUserBannerIDs)

	require.Equal(t, cfgSite.DefaultUserBannerID, firstBannerID, "expected register default banner stay %d, got %d", firstBannerID, cfgSite.DefaultUserBannerID)
}

func checkRegisterDefaultBannerProtected(t *testing.T, fixture *bannerFixture, firstBannerID uint) {
	t.Helper()

	recorder, ctx := newTestContext(http.MethodPut, "/v1/admin/user/banner/defaults")
	ctx.Set("request", AdminUpdateUserBannerListReq{
		BannerIDs: []uint{firstBannerID},
		Action:    domain.UserBannerActionAdd,
	})

	fixture.adminHandler.AdminUpdateUserBannerListView(ctx)

	response := decodeResponse(t, recorder)
	require.EqualValues(t, res.FailServiceCode, response.Code, "expected fail response code, got %d", response.Code)

	require.Equal(t, "默认注册横幅不允许在默认列表中操作", response.Msg, "unexpected response msg: %s", response.Msg)
}

func checkRejectHomeBannerFromUserDefaults(t *testing.T, fixture *bannerFixture, homeBannerID uint) {
	t.Helper()

	recorder, ctx := newTestContext(http.MethodPut, "/v1/admin/user/banner/defaults")
	ctx.Set("request", AdminUpdateUserBannerListReq{
		BannerIDs: []uint{homeBannerID},
		Action:    domain.UserBannerActionAdd,
	})

	fixture.adminHandler.AdminUpdateUserBannerListView(ctx)

	response := decodeResponse(t, recorder)
	require.EqualValues(t, res.FailServiceCode, response.Code, "expected fail response code, got %d", response.Code)

	require.Equal(t, "用户主页横幅不存在", response.Msg, "unexpected response msg: %s", response.Msg)
}
