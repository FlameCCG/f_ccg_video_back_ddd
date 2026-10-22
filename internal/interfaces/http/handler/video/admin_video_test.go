package video

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestAdminListBindsScheduledStateAndExplicitImmediateFilter(t *testing.T) {
	req := httptest.NewRequest("GET", "/admin/video/list?status=5&isScheduled=false", nil)
	var input AdminVideoListReq
	if err := binding.Query.Bind(req, &input); err != nil {
		t.Fatal(err)
	}
	if input.Status != 5 || input.IsScheduled == nil || *input.IsScheduled {
		t.Fatalf("scheduled state or explicit false filter lost: %+v", input)
	}
}
