package dynamic_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	dynamicdomain "f_ccg_video_back_ddd/internal/domain/dynamic"
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
		{name: "未登录", userID: 0, content: "hi", wantErr: dynamicdomain.ErrUserNotLogin},
		{name: "空正文", userID: 1, content: "   ", wantErr: dynamicdomain.ErrContentEmpty},
		{
			name:    "正文超长",
			userID:  1,
			content: strings.Repeat("字", dynamicdomain.MaxContentRunes+1),
			wantErr: dynamicdomain.ErrContentTooLong,
		},
		{
			name:        "边界长度可通过",
			userID:      1,
			content:     strings.Repeat("字", dynamicdomain.MaxContentRunes),
			wantContent: strings.Repeat("字", dynamicdomain.MaxContentRunes),
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

			got, err := dynamicdomain.NewDynamic(tc.userID, tc.content, tc.imageURL)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			if got.Content() != tc.wantContent {
				t.Errorf("content = %q, want %q", got.Content(), tc.wantContent)
			}

			if got.ImageURL() != tc.wantImageURL {
				t.Errorf("imageURL = %q, want %q", got.ImageURL(), tc.wantImageURL)
			}

			if got.ID() != 0 || got.IsPinned() || got.PinnedAt() != nil {
				t.Errorf("new dynamic carries persisted state: id=%d pinned=%v", got.ID(), got.IsPinned())
			}
		})
	}
}

func TestDynamicOwnershipGuards(t *testing.T) {
	t.Parallel()

	d := dynamicdomain.RestoreDynamic(dynamicdomain.DynamicSnapshot{ID: 1, UserID: 42})

	if err := d.EnsureDeletableBy(42); err != nil {
		t.Errorf("owner delete: unexpected err %v", err)
	}

	if err := d.EnsureDeletableBy(7); !errors.Is(err, dynamicdomain.ErrDeleteForbidden) {
		t.Errorf("stranger delete: err = %v, want %v", err, dynamicdomain.ErrDeleteForbidden)
	}

	if err := d.EnsurePinnableBy(42); err != nil {
		t.Errorf("owner pin: unexpected err %v", err)
	}

	if err := d.EnsurePinnableBy(7); !errors.Is(err, dynamicdomain.ErrPinForbidden) {
		t.Errorf("stranger pin: err = %v, want %v", err, dynamicdomain.ErrPinForbidden)
	}

	var missing *dynamicdomain.Dynamic
	if err := missing.EnsureDeletableBy(1); !errors.Is(err, dynamicdomain.ErrDynamicNotFound) {
		t.Errorf("nil delete: err = %v, want %v", err, dynamicdomain.ErrDynamicNotFound)
	}

	if err := missing.EnsurePinnableBy(1); !errors.Is(err, dynamicdomain.ErrDynamicNotFound) {
		t.Errorf("nil pin: err = %v, want %v", err, dynamicdomain.ErrDynamicNotFound)
	}
}

func TestBuildPinPlan(t *testing.T) {
	t.Parallel()

	policy := dynamicdomain.PinPolicy{}
	if plan := policy.BuildPinPlan(true); !plan.ClearOwnPinned || !plan.Pinned {
		t.Errorf("pin plan = %+v, want clear+pin", plan)
	}

	if plan := policy.BuildPinPlan(false); plan.ClearOwnPinned || plan.Pinned {
		t.Errorf("unpin plan = %+v, want no clear, not pinned", plan)
	}

	if err := policy.EnsureVideoPinnable("", false); !errors.Is(err, dynamicdomain.ErrUserNotFound) {
		t.Errorf("empty author: err = %v, want %v", err, dynamicdomain.ErrUserNotFound)
	}

	if err := policy.EnsureVideoPinnable("alice", false); !errors.Is(err, dynamicdomain.ErrVideoPinForbidden) {
		t.Errorf("not owner: err = %v, want %v", err, dynamicdomain.ErrVideoPinForbidden)
	}

	if err := policy.EnsureVideoPinnable("alice", true); err != nil {
		t.Errorf("owner: unexpected err %v", err)
	}
}

func TestReadRecordShouldAdvance(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)

	var missing *dynamicdomain.ReadRecord
	if !missing.ShouldAdvance(base) {
		t.Error("nil record should advance")
	}

	record := dynamicdomain.RestoreReadRecord(dynamicdomain.ReadRecordSnapshot{LastReadWorkAt: base})
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

func TestRestoreDynamicKeepsHistoryAndCopiesPinnedAt(t *testing.T) {
	t.Parallel()

	at := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	raw := at
	snapshot := dynamicdomain.DynamicSnapshot{
		ID:        9,
		CreatedAt: at,
		UpdatedAt: at.Add(time.Minute),
		UserID:    0,
		Content:   "  spaced  ",
		ImageURL:  " img ",
		LikeCount: 4,
		IsPinned:  true,
		PinnedAt:  &raw,
	}

	if _, err := dynamicdomain.NewDynamic(0, "kept", ""); !errors.Is(err, dynamicdomain.ErrUserNotLogin) {
		t.Fatalf("create should still reject user 0, err = %v", err)
	}

	d := dynamicdomain.RestoreDynamic(snapshot)
	raw = raw.Add(time.Hour)
	if d.UserID() != 0 || d.Content() != "  spaced  " || d.ImageURL() != " img " || d.LikeCount() != 4 {
		t.Fatalf("restore revalidated or dropped history: %+v", d.Snapshot())
	}

	if d.PinnedAt() == nil || !d.PinnedAt().Equal(at) || !d.IsPinned() {
		t.Fatalf("pinned state = %v %v", d.IsPinned(), d.PinnedAt())
	}

	viaSnapshot := d.Snapshot()
	*viaSnapshot.PinnedAt = viaSnapshot.PinnedAt.Add(time.Hour)
	if d.PinnedAt() == nil || !d.PinnedAt().Equal(at) {
		t.Fatal("snapshot pinnedAt shares storage with the dynamic")
	}

	returned := d.PinnedAt()
	*returned = returned.Add(time.Hour)
	if !d.PinnedAt().Equal(at) {
		t.Fatal("PinnedAt shares storage with the dynamic")
	}
}

func TestReadRecordAdvanceKeepsStateUntilNewer(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 7, 26, 10, 0, 0, 0, time.UTC)
	record := dynamicdomain.RestoreReadRecord(dynamicdomain.ReadRecordSnapshot{
		ID:               4,
		UserID:           2,
		AuthorID:         9,
		LastReadWorkType: 1,
		LastReadWorkID:   3,
		LastReadWorkAt:   base,
	})

	if record.Advance(2, 8, base) || record.Advance(2, 8, base.Add(-time.Second)) {
		t.Fatal("older or equal work advanced")
	}

	snap := record.Snapshot()
	snap.LastReadWorkID = 99
	if record.LastReadWorkType() != 1 || record.LastReadWorkID() != 3 || !record.LastReadWorkAt().Equal(base) {
		t.Fatalf("failed advance changed state: %+v", record.Snapshot())
	}

	if record.ID() != 4 || record.Snapshot().LastReadWorkID != 3 {
		t.Fatal("snapshot or identity was not copied")
	}

	next := base.Add(time.Second)
	if !record.Advance(2, 8, next) {
		t.Fatal("newer work should advance")
	}

	if record.LastReadWorkType() != 2 || record.LastReadWorkID() != 8 || !record.LastReadWorkAt().Equal(next) {
		t.Fatalf("associated read fields = %+v", record.Snapshot())
	}

	if record.ID() != 4 || record.UserID() != 2 || record.AuthorID() != 9 {
		t.Fatalf("identity changed: %+v", record.Snapshot())
	}

	var missing *dynamicdomain.ReadRecord
	if missing.Advance(2, 8, next) {
		t.Fatal("nil record cannot advance in place")
	}
}
