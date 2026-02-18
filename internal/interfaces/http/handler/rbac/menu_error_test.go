package rbac

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRespondMenuErrorsLogsUnknownFailuresAndPreservesFallbacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wantMsg    string
		wantLogMsg string
		wantFields map[string]string
		respond    func(*Handler, *gin.Context, error)
	}{
		{
			name:       "create",
			wantMsg:    domain.ErrCreateFailed.Error(),
			wantLogMsg: "menu mutation failed",
			wantFields: map[string]string{"operation": "create", "creatorId": "9", "parentMenuId": "3"},
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
			name:       "update",
			wantMsg:    domain.ErrUpdateFailed.Error(),
			wantLogMsg: "menu mutation failed",
			wantFields: map[string]string{"operation": "update", "menuId": "11"},
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
			name:       "move",
			wantMsg:    domain.ErrSortFailed.Error(),
			wantLogMsg: "menu mutation failed",
			wantFields: map[string]string{"operation": "move", "menuId": "11", "targetMenuId": "12"},
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
			name:       "generate",
			wantMsg:    domain.ErrReadMenuDataFailed.Error(),
			wantLogMsg: "menu generation failed",
			wantFields: map[string]string{"operation": "generate", "parentMenuId": "3"},
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
			ctx, recorder := testMenuContext()
			cause := errors.New("database unavailable")
			err := cause
			if tt.name == "generate" {
				err = fmt.Errorf("%w: %w", domain.ErrReadMenuDataFailed, cause)
			}

			tt.respond(handler, ctx, err)
			assertMenuFailureResponse(t, recorder, tt.wantMsg)

			entries := logs.FilterMessage(tt.wantLogMsg).All()
			if len(entries) != 1 {
				t.Fatalf("error log count = %d, want 1", len(entries))
			}
			if entries[0].Level != zap.ErrorLevel {
				t.Fatalf("log level = %s, want error", entries[0].Level)
			}

			fields := entries[0].ContextMap()
			for key, want := range tt.wantFields {
				if got := fmt.Sprint(fields[key]); got != want {
					t.Fatalf("field %q = %q, want %q; fields=%#v", key, got, want, fields)
				}
			}
			if got := fmt.Sprint(fields["error"]); !strings.Contains(got, cause.Error()) {
				t.Fatalf("error field = %q, want cause %q", got, cause)
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
			if logs.Len() != 0 {
				t.Fatalf("known domain failure logged: %#v", logs.All())
			}
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

	if recorder.Code != 200 {
		t.Fatalf("HTTP status = %d, want 200", recorder.Code)
	}

	var response res.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != res.FailServiceCode || response.Msg != wantMsg {
		t.Fatalf("response = %#v, want code=%d msg=%q", response, res.FailServiceCode, wantMsg)
	}
}
