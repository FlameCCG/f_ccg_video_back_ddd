package ai

import (
	"strings"
	"testing"
)

func TestNormalizeAnalysisKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "nil", in: nil, want: nil},
		{name: "trim and drop empty", in: []string{" go ", "", "  "}, want: []string{"go"}},
		{name: "case insensitive dedupe", in: []string{"Go", "go", "GO", "rust"}, want: []string{"Go", "rust"}},
		{name: "drop overlong", in: []string{strings.Repeat("字", 33), "ok"}, want: []string{"ok"}},
		{
			name: "cap at max",
			in:   []string{"a", "b", "c", "d", "e", "f", "g"},
			want: []string{"a", "b", "c", "d", "e", "f"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := (VideoQueryAnalysis{}).NormalizeAnalysisKeywords(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("NormalizeAnalysisKeywords = %#v, want %#v", got, tt.want)
			}

			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("NormalizeAnalysisKeywords[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestVideoQueryAnalysisWantsVideoSearch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		analysis VideoQueryAnalysis
		want     bool
	}{
		{name: "explicit find video", analysis: VideoQueryAnalysis{Intent: "find_video"}, want: true},
		{name: "explicit chat wins", analysis: VideoQueryAnalysis{Intent: "chat", Keywords: []string{"go"}}, want: false},
		{name: "legacy keywords", analysis: VideoQueryAnalysis{Keywords: []string{"go"}}, want: true},
		{name: "empty legacy", analysis: VideoQueryAnalysis{}, want: false},
		{name: "image only intent", analysis: VideoQueryAnalysis{Intent: " FIND_VIDEO "}, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.analysis.WantsVideoSearch(); got != test.want {
				t.Fatalf("WantsVideoSearch() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestTrimPromptText(t *testing.T) {
	t.Parallel()

	if got := (PromptVideoCandidate{}).TrimPromptText("  hello  ", 10); got != "hello" {
		t.Fatalf("TrimPromptText = %q", got)
	}

	if got := (PromptVideoCandidate{}).TrimPromptText("abcdef", 0); got != "abcdef" {
		t.Fatalf("TrimPromptText with zero limit = %q", got)
	}

	if got := (PromptVideoCandidate{}).TrimPromptText("abcdef", 3); got != "abc..." {
		t.Fatalf("TrimPromptText truncated = %q", got)
	}

	if got := (PromptVideoCandidate{}).TrimPromptText(strings.Repeat("字", 4), 4); got != strings.Repeat("字", 4) {
		t.Fatalf("TrimPromptText at limit = %q", got)
	}
}

func TestNewPromptVideoCandidate(t *testing.T) {
	t.Parallel()

	single := NewPromptVideoCandidate(VideoCandidateInput{
		VideoID:     7,
		Title:       " 标题 ",
		Description: strings.Repeat("述", 300),
		Partition:   " 科技 ",
		Cover:       " http://cover ",
		Tags:        []string{" go ", "", "rust"},
		Parts:       []VideoPartInput{{Title: "唯一分P"}},
	})

	if single.WorkType != "single" {
		t.Fatalf("WorkType = %q, want single", single.WorkType)
	}

	if single.Title != "标题" || single.Partition != "科技" || single.Cover != "http://cover" {
		t.Fatalf("candidate not trimmed: %#v", single)
	}

	if len([]rune(single.Description)) != 243 {
		t.Fatalf("Description len = %d, want 240 runes plus ellipsis", len([]rune(single.Description)))
	}

	if len(single.Tags) != 2 || single.Tags[0] != "go" || single.Tags[1] != "rust" {
		t.Fatalf("Tags = %#v", single.Tags)
	}

	if single.PartSummaries != nil {
		t.Fatalf("single work should not carry part summaries: %#v", single.PartSummaries)
	}

	multi := NewPromptVideoCandidate(VideoCandidateInput{
		VideoID: 9,
		Title:   "多P",
		Parts:   []VideoPartInput{{Title: "P1"}, {Title: " P2 "}},
	})

	if multi.WorkType != "multi_part" || len(multi.PartSummaries) != 2 {
		t.Fatalf("multi part candidate = %#v", multi)
	}

	if multi.PartSummaries[1].P != 2 ||
		multi.PartSummaries[1].Title != "P2" {
		t.Fatalf("part summary = %#v", multi.PartSummaries[1])
	}
}

func TestEnsurePositiveLimit(t *testing.T) {
	t.Parallel()

	if got := (PromptVideoCandidateSet{}).EnsurePositiveLimit(0); got != DefaultCandidateLimit {
		t.Fatalf("EnsurePositiveLimit(0) = %d", got)
	}

	if got := (PromptVideoCandidateSet{}).EnsurePositiveLimit(-3); got != DefaultCandidateLimit {
		t.Fatalf("EnsurePositiveLimit(-3) = %d", got)
	}

	if got := (PromptVideoCandidateSet{}).EnsurePositiveLimit(4); got != 4 {
		t.Fatalf("EnsurePositiveLimit(4) = %d", got)
	}
}
