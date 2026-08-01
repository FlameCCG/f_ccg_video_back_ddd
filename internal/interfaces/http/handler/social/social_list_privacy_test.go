package social

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/social"
	domain "f_ccg_video_back_ddd/internal/domain/social"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

type socialListQueryStub struct{}

func (socialListQueryStub) Exists(context.Context, uint, uint) bool { return false }

func (socialListQueryStub) Relation(context.Context, uint, uint) domain.Relation {
	return domain.Relation{}
}

func (socialListQueryStub) Counts(context.Context, uint, bool) domain.RelationCounts {
	return domain.RelationCounts{}
}

func (socialListQueryStub) FollowPrivacy(context.Context, uint) (domain.FollowPrivacy, error) {
	return domain.FollowPrivacy{Exists: true}, nil
}

func (socialListQueryStub) ActionPrivacy(context.Context, uint) (domain.ActionPrivacy, error) {
	return domain.ActionPrivacy{}, nil
}

func (socialListQueryStub) ListFocus(context.Context, uint, int, int, string) ([]app.SocialListUser, int64, error) {
	return []app.SocialListUser{{ID: 2, Username: "followed"}}, 1, nil
}

func (socialListQueryStub) ListFans(context.Context, uint, int, int, string) ([]app.SocialListUser, int64, error) {
	return []app.SocialListUser{{ID: 3, Username: "fan"}}, 1, nil
}

func performSocialList(
	t *testing.T,
	viewerID string,
	handler func(*gin.Context),
) res.Response {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?id=1", nil)
	if viewerID != "" {
		ctx.Set("uid", viewerID)
	}

	handler(ctx)

	var response res.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	return response
}

func TestSocialListOwnerBypassesClosedPrivacy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	relations := app.NewRelationService(socialListQueryStub{})
	handler := NewHandler(nil, relations)

	tests := []struct {
		name       string
		handler    func(*gin.Context)
		blockedMsg string
	}{
		{name: "follow list", handler: handler.FocusListView, blockedMsg: "用户未公开关注"},
		{name: "fans list", handler: handler.FansListView, blockedMsg: "用户未公开粉丝"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ownerResponse := performSocialList(t, "1", test.handler)
			if ownerResponse.Code != 0 {
				t.Fatalf("owner response code=%d msg=%q", ownerResponse.Code, ownerResponse.Msg)
			}

			otherResponse := performSocialList(t, "2", test.handler)
			if otherResponse.Code == 0 || otherResponse.Msg != test.blockedMsg {
				t.Fatalf("other response code=%d msg=%q, want %q", otherResponse.Code, otherResponse.Msg, test.blockedMsg)
			}

			anonymousResponse := performSocialList(t, "", test.handler)
			if anonymousResponse.Code == 0 {
				t.Fatal("anonymous viewer should be blocked by privacy")
			}
		})
	}
}
