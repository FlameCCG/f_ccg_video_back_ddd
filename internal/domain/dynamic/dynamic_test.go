package dynamic_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/dynamic"
)

func TestNewDynamic(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		userID       uint
		content      string
		imageURL     string
		wantErr      error
		wantContent  string
		wantImageURL string
	}{
		{name: "未登录", userID: 0, content: "hi", wantErr: domain.ErrUserNotLogin},
		{name: "空正文", userID: 1, content: "   ", wantErr: domain.ErrContentEmpty},
		{
			name:    "正文超长",
			userID:  1,
			content: strings.Repeat("字", domain.MaxContentRunes+1),
			wantErr: domain.ErrContentTooLong,
		},
		{
			name:        "边界长度可通过",
			userID:      1,
			content:     strings.Repeat("字", domain.MaxContentRunes),
			wantContent: strings.Repeat("字", domain.MaxContentRunes),
		},
		{
			name:         "裁剪首尾空白",
			userID:       1,
			content:      "  今天天气不错  ",
			imageURL:     "  https://example.com/a.png ",
			wantContent:  "今天天气不错",
			wantImageURL: "https://example.com/a.png",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.NewDynamic(tc.userID, tc.content, tc.imageURL)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			if got.Content != tc.wantContent {
				t.Errorf("content = %q, want %q", got.Content, tc.wantContent)
			}

			if got.ImageURL != tc.wantImageURL {
				t.Errorf("imageURL = %q, want %q", got.ImageURL, tc.wantImageURL)
			}
		})
	}
}

func TestDynamicOwnershipGuards(t *testing.T) {
	t.Parallel()

	d := &domain.Dynamic{ID: 1, UserID: 42}

	if err := d.EnsureDeletableBy(42); err != nil {
		t.Errorf("owner delete: unexpected err %v", err)
	}

	if err := d.EnsureDeletableBy(7); !errors.Is(err, domain.ErrDeleteForbidden) {
		t.Errorf("stranger delete: err = %v, want %v", err, domain.ErrDeleteForbidden)
	}

	if err := d.EnsurePinnableBy(42); err != nil {
		t.Errorf("owner pin: unexpected err %v", err)
	}

	if err := d.EnsurePinnableBy(7); !errors.Is(err, domain.ErrPinForbidden) {
		t.Errorf("stranger pin: err = %v, want %v", err, domain.ErrPinForbidden)
	}

	var missing *domain.Dynamic
	if err := missing.EnsureDeletableBy(1); !errors.Is(err, domain.ErrDynamicNotFound) {
		t.Errorf("nil delete: err = %v, want %v", err, domain.ErrDynamicNotFound)
	}

	if err := missing.EnsurePinnableBy(1); !errors.Is(err, domain.ErrDynamicNotFound) {
		t.Errorf("nil pin: err = %v, want %v", err, domain.ErrDynamicNotFound)
	}
}

func TestBuildPinPlan(t *testing.T) {
	t.Parallel()

	if plan := domain.BuildPinPlan(true); !plan.ClearOwnPinned || !plan.Pinned {
		t.Errorf("pin plan = %+v, want clear+pin", plan)
	}

	if plan := domain.BuildPinPlan(false); plan.ClearOwnPinned || plan.Pinned {
		t.Errorf("unpin plan = %+v, want no clear, not pinned", plan)
	}
}

func TestReadRecordShouldAdvance(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)

	var missing *domain.ReadRecord
	if !missing.ShouldAdvance(base) {
		t.Error("nil record should advance")
	}

	record := &domain.ReadRecord{LastReadWorkAt: base}
	if record.ShouldAdvance(base) {
		t.Error("equal time should not advance")
	}

	if record.ShouldAdvance(base.Add(-time.Second)) {
		t.Error("older work should not advance")
	}

	if !record.ShouldAdvance(base.Add(time.Second)) {
		t.Error("newer work should advance")
	}
}
