package identity

import (
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"f_ccg_video_back_ddd/pkg/res"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateUserConfView(t *testing.T) {
	t.Parallel()

	fixture := newBannerFixture(t, &config.Config{
		Site: &site.Site{
			DefaultUserBannerID:  19,
			DefaultUserBannerIDs: []uint{21},
		},
	}, writeTempConfig(t, "site:\n  defaultuserbannerid: 19\n"))

	seedUserConf(t, fixture.db, po.UserConf{
		UserID:    1,
		BannerID:  19,
		BannerURL: testCustomBannerURL,
		LikeTags:  []byte(`["golang","music"]`),
	})

	allowedBannerID := seedBanner(t, fixture.db, &po.Banner{
		Model: po.Model{ID: 21},
		Cover: "https://cdn.example.com/banner-user-21.jpg",
		Show:  true,
		Type:  enum.BannerTypeUser,
	})

	registerDefaultBannerID := seedBanner(t, fixture.db, &po.Banner{
		Model: po.Model{ID: 19},
		Cover: "https://cdn.example.com/banner-user-19.jpg",
		Show:  true,
		Type:  enum.BannerTypeUser,
	})

	otherBannerID := seedBanner(t, fixture.db, &po.Banner{
		Model: po.Model{ID: 22},
		Cover: "https://cdn.example.com/banner-user-22.jpg",
		Show:  true,
		Type:  enum.BannerTypeUser,
	})

	//nolint:paralleltest // Subtests exercise a shared parent fixture in sequence.
	t.Run("set system banner clears custom url", func(t *testing.T) {
		recorder, ctx := newTestContext(http.MethodPut, "/v1/common/user/info/conf")
		ctx.Set("uid", "1")
		ctx.Set("request", UpdateUserConfReq{
			BannerID:  &allowedBannerID,
			BannerURL: "https://cdn.example.com/ignored.jpg",
		})

		fixture.handler.UpdateUserConfView(ctx)

		require.Equal(t, http.StatusOK, recorder.Code, "unexpected status code: %d", recorder.Code)

		response := decodeResponse(t, recorder)
		require.EqualValues(t, res.SuccessCode, response.Code, "unexpected response code: %d (%s)", response.Code, response.Msg)

		conf := fixture.userConf(t, 1)
		require.Equal(t, conf.BannerID, allowedBannerID, "expected banner id %d, got %d", allowedBannerID, conf.BannerID)

		require.Empty(t, conf.BannerURL, "expected banner url cleared, got %q", conf.BannerURL)
	})

	//nolint:paralleltest // Subtests exercise a shared parent fixture in sequence.
	t.Run("allow register default banner", func(t *testing.T) {
		recorder, ctx := newTestContext(http.MethodPut, "/v1/common/user/info/conf")
		ctx.Set("uid", "1")
		ctx.Set("request", UpdateUserConfReq{BannerID: &registerDefaultBannerID})

		fixture.handler.UpdateUserConfView(ctx)

		require.Equal(t, http.StatusOK, recorder.Code, "unexpected status code: %d", recorder.Code)

		response := decodeResponse(t, recorder)
		require.EqualValues(t, res.SuccessCode, response.Code, "unexpected response code: %d (%s)", response.Code, response.Msg)

		conf := fixture.userConf(t, 1)
		require.Equal(t, conf.BannerID, registerDefaultBannerID, "expected banner id %d, got %d", registerDefaultBannerID, conf.BannerID)

		require.Empty(t, conf.BannerURL, "expected banner url cleared, got %q", conf.BannerURL)
	})

	//nolint:paralleltest // Subtests exercise a shared parent fixture in sequence.
	t.Run("reject non-system banner", func(t *testing.T) {
		recorder, ctx := newTestContext(http.MethodPut, "/v1/common/user/info/conf")
		ctx.Set("uid", "1")
		ctx.Set("request", UpdateUserConfReq{BannerID: &otherBannerID})

		fixture.handler.UpdateUserConfView(ctx)

		response := decodeResponse(t, recorder)
		require.EqualValues(t, res.FailServiceCode, response.Code, "expected fail response code, got %d", response.Code)

		require.Equal(t, "系统默认横幅不存在", response.Msg, "unexpected response msg: %s", response.Msg)
	})

	//nolint:paralleltest // Subtests exercise a shared parent fixture in sequence.
	t.Run("allow clearing like tags", func(t *testing.T) {
		recorder, ctx := newTestContext(http.MethodPut, "/v1/common/user/info/conf")
		ctx.Set("uid", "1")

		emptyTags := []string{}
		ctx.Set("request", UpdateUserConfReq{LikeTags: &emptyTags})

		fixture.handler.UpdateUserConfView(ctx)

		require.Equal(t, http.StatusOK, recorder.Code, "unexpected status code: %d", recorder.Code)

		{
			got := string(fixture.userConf(t, 1).LikeTags)
			require.Equal(t, "[]", got, "expected like tags to be cleared, got %q", got)
		}
	})
}

// TestGetUserConfViewKeepsRawLikeTagsJSON 守住 GET /user/info/conf 的 likeTags 逐字契约：
// 数据库中的原始 JSON 必须原样输出，而不是被编码成 base64（[]byte 的默认 JSON 行为）。
func TestGetUserConfViewKeepsRawLikeTagsJSON(t *testing.T) {
	t.Parallel()

	fixture := newBannerFixture(t, &config.Config{Site: &site.Site{}}, writeTempConfig(t, "site:\n"))

	seedUserConf(t, fixture.db, po.UserConf{
		UserID:      5,
		BannerID:    19,
		LikeTags:    []byte(`["golang","music"]`),
		OpenCollect: true,
	})

	recorder, ctx := newTestContext(http.MethodGet, "/v1/common/user/info/conf")
	ctx.Set("uid", "5")

	fixture.handler.GetUserConfView(ctx)

	response := decodeResponse(t, recorder)
	require.EqualValues(t, res.SuccessCode, response.Code, "unexpected response code: %d (%s)", response.Code, response.Msg)

	var conf struct {
		LikeTags  []string `json:"likeTags"`
		BannerID  uint     `json:"bannerId"`
		BannerURL string   `json:"bannerUrl"`
	}
	decodeResponseData(t, response, &conf)

	require.Len(t, conf.LikeTags, 2, "expected raw like tags array, got %+v", conf.LikeTags)
	require.Equal(t, "golang", conf.LikeTags[0], "expected raw like tags array, got %+v", conf.LikeTags)
	require.Equal(t, "music", conf.LikeTags[1], "expected raw like tags array, got %+v", conf.LikeTags)

	require.EqualValues(t, 19, conf.BannerID, "expected banner id 19, got %d", conf.BannerID)
}
