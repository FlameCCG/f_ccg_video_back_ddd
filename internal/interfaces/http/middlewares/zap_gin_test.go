package middlewares

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/logmeta"
	"f_ccg_video_back_ddd/pkg/res"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/identity"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestLoggingBoundaryContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		handler gin.HandlerFunc
		details int
		success any
	}{
		{"success", func(c *gin.Context) { res.OkWithMsg("ok", c) }, 0, true},
		{"rejection", func(c *gin.Context) {
			res.FailWithCause(domain.ErrUserNotFound, clienterror.Message(domain.ErrUserNotFound), c)
		}, 0, false},
		{"validation", func(c *gin.Context) { res.FailWithError(errors.New("bad parameter"), c) }, 0, false},
		{"system", func(c *gin.Context) {
			err := apperr.Wrap("lookup", domain.ErrPasswordHashFailed, errors.New("backend unavailable"))
			res.FailWithCause(err, clienterror.Message(err), c)
		}, 1, false},
		{"nil cause", func(c *gin.Context) { res.FailWithCause(nil, "failed", c) }, 1, false},
		{"unmanaged", func(c *gin.Context) { c.String(200, "plain") }, 0, nil},
		{"panic", func(*gin.Context) { panic("failed") }, 1, false},
		{"panic after refusal", func(c *gin.Context) { res.FailWithMsg("no", c); panic("late panic") }, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			core, logs := observer.New(zap.DebugLevel)
			logger := zap.New(core)
			engine := gin.New()
			engine.Use(RequestContext(), GinLogger(logger), GinRecovery(logger, true))
			engine.GET("/resource/:id", tt.handler)

			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/resource/private-id?token=must-not-log&text=private-chat", nil)
			request.Header.Set("X-Request-ID", "untrusted")
			request.Header.Set("Cookie", "session=private-cookie")

			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)

			{
				id := recorder.Header().Get("X-Request-ID")
				require.NotEmpty(t, id, "invalid server request ID: %q", id)
				require.NotEqual(t, "untrusted", id, "invalid server request ID: %q", id)
			}

			summaries := logs.FilterMessage("http.request").All()
			require.Len(t, summaries, 1, "summary count=%d", len(summaries))

			fields := summaries[0].ContextMap()

			got, exists := fields["success"]
			if tt.success == nil {
				require.False(t, exists, "unmanaged response inferred as success")
			} else {
				require.EqualValues(t, got, tt.success, "success=%v want %v", got, tt.success)
			}

			require.EqualValues(t, "/resource/:id", fields["route"], "route=%v", fields["route"])

			{
				_, ok := fields["biz_code"]
				require.False(t, ok, "response code leaked into logging contract")
			}

			{
				_, ok := fields["error"]
				require.False(t, ok, "summary repeats exception detail")
			}

			details := logs.FilterLevelExact(zap.ErrorLevel).All()
			require.Equal(t, len(details), tt.details, "exception count=%d want=%d logs=%#v", len(details), tt.details, logs.All())

			for _, entry := range logs.All() {
				for _, field := range entry.Context {
					text := field.String
					require.NotContains(t, text, "must-not-log", "sensitive request material logged: %s", field.Key)
					require.NotContains(t, text, "private-chat", "sensitive request material logged: %s", field.Key)
					require.NotContains(t, text, "private-cookie", "sensitive request material logged: %s", field.Key)
					require.NotContains(t, text, "private-id", "sensitive request material logged: %s", field.Key)
				}
			}

			if tt.name == "panic" {
				require.Equal(t, 500, recorder.Code, "panic lost HTTP behavior or stack")
				require.NotNil(t, details[0].ContextMap()["stack"], "panic lost HTTP behavior or stack")
			}

			if tt.name == "system" {
				require.False(t, recorder.Code != 200 || !strings.Contains(recorder.Body.String(), `"code":1`) || strings.Contains(recorder.Body.String(), "backend unavailable"), "response contract changed: %s", recorder.Body.String())
			}
		})
	}
}

func TestCancellationRequiresRequestEvidence(t *testing.T) {
	t.Parallel()

	for _, clientCanceled := range []bool{true, false} {
		t.Run(map[bool]string{true: "client", false: "dependency"}[clientCanceled], func(t *testing.T) {
			t.Parallel()

			core, logs := observer.New(zap.DebugLevel)
			lg := zap.New(core)
			engine := gin.New()
			engine.Use(RequestContext(), GinLogger(lg), GinRecovery(lg, true))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			engine.GET("/stream", func(c *gin.Context) {
				if clientCanceled {
					cancel()
				}

				res.RecordFailure(context.Canceled, c)
			})

			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/stream", nil).WithContext(ctx))

			want := 1
			if clientCanceled {
				want = 0
			}

			{
				got := logs.FilterLevelExact(zap.ErrorLevel).Len()
				require.Equal(t, want, got, "Error count=%d want=%d", got, want)
			}
		})
	}
}

func TestRequestMetadataReachesApplicationContext(t *testing.T) {
	t.Parallel()

	engine := gin.New()
	engine.Use(RequestContext())
	engine.GET("/", func(c *gin.Context) {
		require.Equal(t, logmeta.From(c.Request.Context()).RequestID, c.Writer.Header().Get("X-Request-ID"), "context and response correlation differ")
	})
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
}
