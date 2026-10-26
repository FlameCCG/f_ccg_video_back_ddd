package rbac

import (
	"encoding/json"
	"errors"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRespondMenuErrorsLogsUnknownFailuresAndPreservesFallbacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantMsg string
		respond func(*Handler, *gin.Context, error)
	}{
		{
			name:    "create",
			wantMsg: domain.ErrCreateFailed.Error(),
			respond: func(h *Handler, c *gin.Context, err error) {
				parentID := uint(3)
				h.respondMenuMutationError(
					c,
					"create",
					err,
					domain.ErrCreateFailed,
					zap.Uint("creatorId", 9),
					optionalMenuIDField("parentMenuId", &parentID),
				)
			},
		},
		{
			name:    "update",
			wantMsg: domain.ErrUpdateFailed.Error(),
			respond: func(h *Handler, c *gin.Context, err error) {
				h.respondMenuMutationError(
					c,
					"update",
					err,
					domain.ErrUpdateFailed,
					zap.Uint("menuId", 11),
				)
			},
		},
		{
			name:    "move",
			wantMsg: domain.ErrSortFailed.Error(),
			respond: func(h *Handler, c *gin.Context, err error) {
				h.respondMenuMutationError(
					c,
					"move",
					err,
					domain.ErrSortFailed,
					zap.Uint("menuId", 11),
					zap.Uint("targetMenuId", 12),
				)
			},
		},
		{
			name:    "generate",
			wantMsg: domain.ErrReadMenuDataFailed.Error(),
			respond: func(h *Handler, c *gin.Context, err error) {
				parentID := uint(3)
				h.respondMenuGenerationError(c, err, &parentID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			core, logs := observer.New(zap.DebugLevel)
			handler := NewHandler(nil, nil, nil, nil, zap.New(core))
			recorder := httptest.NewRecorder()
			cause := errors.New("database unavailable")

			err := cause
			if tt.name == "generate" {
				err = fmt.Errorf("%w: %w", domain.ErrReadMenuDataFailed, cause)
			}

			engine := gin.New()
			engine.Use(middlewares.RequestContext(), middlewares.GinLogger(zap.New(core)), middlewares.GinRecovery(zap.New(core), true))
			engine.POST("/menu", func(ctx *gin.Context) { tt.respond(handler, ctx, err) })
			engine.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/menu", nil))
			assertMenuFailureResponse(t, recorder, tt.wantMsg)

			entries := logs.FilterMessage("http.request.failed").All()
			require.Len(t, entries, 1, "error log count = %d, want 1", len(entries))

			require.Equal(t, zap.ErrorLevel, entries[0].Level, "log level = %s, want error", entries[0].Level)

			fields := entries[0].ContextMap()
			{
				got := fields["operation"]
				require.EqualValues(t, "POST /menu", got, "operation=%v", got)
			}

			require.Equal(t, 1, logs.FilterMessage("http.request").Len(), "missing request summary")

			{
				got := fmt.Sprint(fields["error"])
				require.Contains(t, got, cause.Error(), "error field = %q, want cause %q", got, cause)
			}
		})
	}
}

func TestRespondMenuErrorsDoNotErrorLogKnownDomainFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantMsg string
		respond func(*Handler, *gin.Context)
	}{
		{
			name:    "mutation",
			wantMsg: domain.ErrMenuNotFound.Error(),
			respond: func(h *Handler, c *gin.Context) {
				h.respondMenuMutationError(
					c,
					"update",
					fmt.Errorf("lookup: %w", domain.ErrMenuNotFound),
					domain.ErrUpdateFailed,
					zap.Uint("menuId", 99),
				)
			},
		},
		{
			name:    "mutation validation",
			wantMsg: domain.ErrMenuTitleEmpty.Error(),
			respond: func(h *Handler, c *gin.Context) {
				h.respondMenuMutationError(
					c,
					"update",
					domain.ErrMenuTitleEmpty,
					domain.ErrUpdateFailed,
					zap.Uint("menuId", 99),
				)
			},
		},
		{
			name:    "generation",
			wantMsg: domain.ErrParentMenuNotFound.Error(),
			respond: func(h *Handler, c *gin.Context) {
				h.respondMenuGenerationError(c, domain.ErrParentMenuNotFound, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			core, logs := observer.New(zap.DebugLevel)
			handler := NewHandler(nil, nil, nil, nil, zap.New(core))
			ctx, recorder := testMenuContext()

			tt.respond(handler, ctx)
			assertMenuFailureResponse(t, recorder, tt.wantMsg)

			require.Equal(t, 0, logs.Len(), "known domain failure logged: %#v", logs.All())
		})
	}
}

func TestRespondMenuMutationErrorUsesNopLoggerWhenNil(t *testing.T) {
	t.Parallel()

	handler := NewHandler(nil, nil, nil, nil, nil)
	ctx, recorder := testMenuContext()
	handler.respondMenuMutationError(
		ctx,
		"update",
		errors.New("database unavailable"),
		domain.ErrUpdateFailed,
		zap.Uint("menuId", 1),
	)

	assertMenuFailureResponse(t, recorder, domain.ErrUpdateFailed.Error())
}

func testMenuContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	return ctx, recorder
}

func assertMenuFailureResponse(t *testing.T, recorder *httptest.ResponseRecorder, wantMsg string) {
	t.Helper()

	require.Equal(t, 200, recorder.Code, "HTTP status = %d, want 200", recorder.Code)

	var response res.Response
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err, "decode response: %v", err)
	}

	require.EqualValues(t, res.FailServiceCode, response.Code, "response = %#v, want code=%d msg=%q", response, res.FailServiceCode, wantMsg)
	require.Equal(t, wantMsg, response.Msg, "response = %#v, want code=%d msg=%q", response, res.FailServiceCode, wantMsg)
}
