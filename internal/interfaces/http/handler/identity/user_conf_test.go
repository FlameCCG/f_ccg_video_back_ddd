package identity

import (
	"net/http"
	"testing"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/config/site"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

func TestUpdateUserConfView(t *testing.T) {
	gin.SetMode(gin.TestMode)

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

	t.Run("set system banner clears custom url", func(t *testing.T) {
		recorder, ctx := newTestContext(http.MethodPut, "/v1/common/user/info/conf")
		ctx.Set("uid", "1")
		ctx.Set("request", UpdateUserConfReq{
			BannerID:  &allowedBannerID,
			BannerURL: "https://cdn.example.com/ignored.jpg",
		})

		fixture.handler.UpdateUserConfView(ctx)

		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status code: %d", recorder.Code)
		}

		response := decodeResponse(t, recorder)
		if response.Code != res.SuccessCode {
			t.Fatalf("unexpected response code: %d (%s)", response.Code, response.Msg)
		}

		conf := fixture.userConf(t, 1)
		if conf.BannerID != allowedBannerID {
			t.Fatalf("expected banner id %d, got %d", allowedBannerID, conf.BannerID)
		}

		if conf.BannerURL != "" {
			t.Fatalf("expected banner url cleared, got %q", conf.BannerURL)
		}
	})

	t.Run("allow register default banner", func(t *testing.T) {
		recorder, ctx := newTestContext(http.MethodPut, "/v1/common/user/info/conf")
		ctx.Set("uid", "1")
		ctx.Set("request", UpdateUserConfReq{BannerID: &registerDefaultBannerID})

		fixture.handler.UpdateUserConfView(ctx)

		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status code: %d", recorder.Code)
		}

		response := decodeResponse(t, recorder)
		if response.Code != res.SuccessCode {
			t.Fatalf("unexpected response code: %d (%s)", response.Code, response.Msg)
		}

		conf := fixture.userConf(t, 1)
		if conf.BannerID != registerDefaultBannerID {
			t.Fatalf("expected banner id %d, got %d", registerDefaultBannerID, conf.BannerID)
		}

		if conf.BannerURL != "" {
			t.Fatalf("expected banner url cleared, got %q", conf.BannerURL)
		}
	})

	t.Run("reject non-system banner", func(t *testing.T) {
		recorder, ctx := newTestContext(http.MethodPut, "/v1/common/user/info/conf")
		ctx.Set("uid", "1")
		ctx.Set("request", UpdateUserConfReq{BannerID: &otherBannerID})

		fixture.handler.UpdateUserConfView(ctx)

		response := decodeResponse(t, recorder)
		if response.Code != res.FailServiceCode {
			t.Fatalf("expected fail response code, got %d", response.Code)
		}

		if response.Msg != "系统默认横幅不存在" {
			t.Fatalf("unexpected response msg: %s", response.Msg)
		}
	})

	t.Run("allow clearing like tags", func(t *testing.T) {
		recorder, ctx := newTestContext(http.MethodPut, "/v1/common/user/info/conf")
		ctx.Set("uid", "1")

		emptyTags := []string{}
		ctx.Set("request", UpdateUserConfReq{LikeTags: &emptyTags})

		fixture.handler.UpdateUserConfView(ctx)

		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status code: %d", recorder.Code)
		}

		if got := string(fixture.userConf(t, 1).LikeTags); got != "[]" {
			t.Fatalf("expected like tags to be cleared, got %q", got)
		}
	})
}

// TestGetUserConfViewKeepsRawLikeTagsJSON 守住 GET /user/info/conf 的 likeTags 逐字契约：
// 数据库中的原始 JSON 必须原样输出，而不是被编码成 base64（[]byte 的默认 JSON 行为）。
func TestGetUserConfViewKeepsRawLikeTagsJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
	if response.Code != res.SuccessCode {
		t.Fatalf("unexpected response code: %d (%s)", response.Code, response.Msg)
	}

	var conf struct {
		LikeTags  []string `json:"likeTags"`
		BannerID  uint     `json:"bannerId"`
		BannerURL string   `json:"bannerUrl"`
	}
	decodeResponseData(t, response, &conf)

	if len(conf.LikeTags) != 2 || conf.LikeTags[0] != "golang" || conf.LikeTags[1] != "music" {
		t.Fatalf("expected raw like tags array, got %+v", conf.LikeTags)
	}

	if conf.BannerID != 19 {
		t.Fatalf("expected banner id 19, got %d", conf.BannerID)
	}
}
