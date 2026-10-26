package video

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin/binding"
	"github.com/stretchr/testify/require"
)

func TestAdminListBindsScheduledStateAndExplicitImmediateFilter(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/video/list?status=5&isScheduled=false", nil)

	var input AdminVideoListReq
	{
		err := binding.Query.Bind(req, &input)
		require.NoError(t, err)
	}

	require.Equal(t, 5, input.Status, "scheduled state or explicit false filter lost: %+v", input)
	require.NotNil(t, input.IsScheduled, "scheduled state or explicit false filter lost: %+v", input)
	require.False(t, *input.IsScheduled, "scheduled state or explicit false filter lost: %+v", input)
}
