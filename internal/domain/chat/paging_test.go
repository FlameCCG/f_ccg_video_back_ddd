package chat

import (
	"errors"
	"testing"
)

// TestNormalizeConversationPage 迁移自原 service/chat_service/page_test.go。
func TestNormalizeConversationPage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		page         int
		pageSize     int
		wantPage     int
		wantPageSize int
	}{
		{name: "defaults", page: 0, pageSize: 0, wantPage: 1, wantPageSize: DefaultConversationPageSize},
		{name: "normal", page: 2, pageSize: 10, wantPage: 2, wantPageSize: 10},
		{name: "clamp huge pageSize", page: 1, pageSize: 1_000_000, wantPage: 1, wantPageSize: MaxConversationPageSize},
		{name: "at max", page: 1, pageSize: MaxConversationPageSize, wantPage: 1, wantPageSize: MaxConversationPageSize},
		{name: "negative", page: -1, pageSize: -5, wantPage: 1, wantPageSize: DefaultConversationPageSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotPage, gotSize := NormalizeConversationPage(tt.page, tt.pageSize)
			if gotPage != tt.wantPage || gotSize != tt.wantPageSize {
				t.Fatalf("NormalizeConversationPage(%d, %d) = (%d, %d), want (%d, %d)",
					tt.page, tt.pageSize, gotPage, gotSize, tt.wantPage, tt.wantPageSize)
			}
		})
	}
}

func TestValidateMessageCursor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		before  string
		wantErr error
	}{
		{name: "empty means no cursor", before: "", wantErr: nil},
		{name: "valid lowercase hex", before: "652f1a2b3c4d5e6f70819200", wantErr: nil},
		{name: "valid uppercase hex", before: "652F1A2B3C4D5E6F70819200", wantErr: nil},
		{name: "too short", before: "652f1a2b3c4d5e6f708192", wantErr: ErrInvalidCursor},
		{name: "too long", before: "652f1a2b3c4d5e6f708192001", wantErr: ErrInvalidCursor},
		{name: "non hex char", before: "652f1a2b3c4d5e6f7081920z", wantErr: ErrInvalidCursor},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateMessageCursor(tt.before); !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateMessageCursor(%q) = %v, want %v", tt.before, err, tt.wantErr)
			}
		})
	}
}

func TestTrimMessagePage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       []int
		want        []int
		wantHasMore bool
	}{
		{name: "empty", input: []int{}, want: []int{}, wantHasMore: false},
		{name: "under page size", input: []int{3, 2, 1}, want: []int{1, 2, 3}, wantHasMore: false},
		{
			name:        "exactly page size",
			input:       []int{10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
			want:        []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			wantHasMore: false,
		},
		{
			name:        "one extra row means has more",
			input:       []int{11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
			want:        []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
			wantHasMore: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, hasMore := TrimMessagePage(tt.input)
			if hasMore != tt.wantHasMore {
				t.Fatalf("TrimMessagePage hasMore = %v, want %v", hasMore, tt.wantHasMore)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("TrimMessagePage len = %d, want %d", len(got), len(tt.want))
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("TrimMessagePage[%d] = %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}
