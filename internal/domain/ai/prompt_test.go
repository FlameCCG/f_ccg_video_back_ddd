package ai

import (
	"encoding/json"
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

			got := NormalizeAnalysisKeywords(tt.in)
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

func TestBuildVideoRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		videoID uint
		p       int
		want    string
	}{
		{name: "zero id", videoID: 0, p: 1, want: ""},
		{name: "single", videoID: 12, p: 0, want: "/video/12"},
		{name: "multi part", videoID: 12, p: 3, want: "/video/12/3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := BuildVideoRoute(tt.videoID, tt.p); got != tt.want {
				t.Fatalf("BuildVideoRoute(%d, %d) = %q, want %q", tt.videoID, tt.p, got, tt.want)
			}
		})
	}
}

func TestTrimPromptText(t *testing.T) {
	t.Parallel()

	if got := TrimPromptText("  hello  ", 10); got != "hello" {
		t.Fatalf("TrimPromptText = %q", got)
	}

	if got := TrimPromptText("abcdef", 0); got != "abcdef" {
		t.Fatalf("TrimPromptText with zero limit = %q", got)
	}

	if got := TrimPromptText("abcdef", 3); got != "abc..." {
		t.Fatalf("TrimPromptText truncated = %q", got)
	}

	if got := TrimPromptText(strings.Repeat("字", 4), 4); got != strings.Repeat("字", 4) {
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

	if single.Route != "/video/7" {
		t.Fatalf("Route = %q", single.Route)
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
		multi.PartSummaries[1].Title != "P2" ||
		multi.PartSummaries[1].Route != "/video/9/2" {
		t.Fatalf("part summary = %#v", multi.PartSummaries[1])
	}
}

func TestBuildPromptVideoContext(t *testing.T) {
	t.Parallel()

	empty, err := BuildPromptVideoContext(8, PromptVideoCandidateSet{})
	if err != nil {
		t.Fatalf("BuildPromptVideoContext error: %v", err)
	}

	if !strings.Contains(empty, "当前候选来源：没有候选作品。") {
		t.Fatalf("empty context missing default source line: %q", empty)
	}

	if !strings.Contains(empty, "本轮作品候选上限：8") {
		t.Fatalf("empty context missing limit line: %q", empty)
	}

	if !strings.Contains(empty, "最多返回 5 个链接结果") {
		t.Fatalf("empty context missing link cap: %q", empty)
	}

	modes := map[string]string{
		RetrievalModeVector:  "语义向量召回结果",
		RetrievalModeKeyword: "关键词命中结果",
		RetrievalModeLatest:  "按最新发布时间回退",
	}

	for mode, marker := range modes {
		got, err := BuildPromptVideoContext(8, PromptVideoCandidateSet{
			RetrievalMode: mode,
			QueryKeywords: []string{"go", "并发"},
			Candidates:    []PromptVideoCandidate{{VideoID: 1, Title: "T", Route: "/video/1", WorkType: "single"}},
		})
		if err != nil {
			t.Fatalf("BuildPromptVideoContext(%s) error: %v", mode, err)
		}

		if !strings.Contains(got, marker) {
			t.Fatalf("context for %s missing %q: %q", mode, marker, got)
		}

		if !strings.Contains(got, "第一轮 AI 提炼出的搜索关键词：go、并发") {
			t.Fatalf("context for %s missing keywords: %q", mode, got)
		}

		if !strings.Contains(got, `"video_id": 1`) {
			t.Fatalf("context for %s missing candidate JSON: %q", mode, got)
		}
	}
}

func TestPromptVideoCandidateJSONShape(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(PromptVideoCandidate{VideoID: 3, Title: "T", Route: "/video/3", WorkType: "single"})
	if err != nil {
		t.Fatalf("marshal candidate: %v", err)
	}

	want := `{"video_id":3,"title":"T","route":"/video/3","work_type":"single"}`
	if string(raw) != want {
		t.Fatalf("candidate JSON = %s, want %s", raw, want)
	}
}

func TestEnsurePositiveLimit(t *testing.T) {
	t.Parallel()

	if got := EnsurePositiveLimit(0); got != DefaultCandidateLimit {
		t.Fatalf("EnsurePositiveLimit(0) = %d", got)
	}

	if got := EnsurePositiveLimit(-3); got != DefaultCandidateLimit {
		t.Fatalf("EnsurePositiveLimit(-3) = %d", got)
	}

	if got := EnsurePositiveLimit(4); got != 4 {
		t.Fatalf("EnsurePositiveLimit(4) = %d", got)
	}
}
