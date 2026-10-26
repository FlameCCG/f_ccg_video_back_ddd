package ai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
		{name: "case insensitive dedupe", in: []string{"Go", "go", "GO", testRust}, want: []string{"Go", testRust}},
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
			require.Len(t, tt.want, len(got), "NormalizeAnalysisKeywords = %#v, want %#v", got, tt.want)

			for i := range tt.want {
				require.Equal(t, got[i], tt.want[i], "NormalizeAnalysisKeywords[%d] = %q, want %q", i, got[i], tt.want[i])
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

			{
				got := test.analysis.WantsVideoSearch()
				require.Equal(t, test.want, got, "WantsVideoSearch() = %v, want %v", got, test.want)
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
		{name: testSingle, videoID: 12, p: 0, want: "/video/12"},
		{name: "multi part", videoID: 12, p: 3, want: "/video/12/3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			{
				got := BuildVideoRoute(tt.videoID, tt.p)
				require.Equal(t, tt.want, got, "BuildVideoRoute(%d, %d) = %q, want %q", tt.videoID, tt.p, got, tt.want)
			}
		})
	}
}

func TestTrimPromptText(t *testing.T) {
	t.Parallel()

	{
		got := TrimPromptText("  hello  ", 10)
		require.Equal(t, "hello", got, "TrimPromptText = %q", got)
	}

	{
		got := TrimPromptText("abcdef", 0)
		require.Equal(t, "abcdef", got, "TrimPromptText with zero limit = %q", got)
	}

	{
		got := TrimPromptText("abcdef", 3)
		require.Equal(t, "abc...", got, "TrimPromptText truncated = %q", got)
	}

	{
		got := TrimPromptText(strings.Repeat("字", 4), 4)
		require.Equal(t, got, strings.Repeat("字", 4), "TrimPromptText at limit = %q", got)
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
		Tags:        []string{" go ", "", testRust},
		Parts:       []VideoPartInput{{Title: "唯一分P"}},
	})

	require.Equal(t, testSingle, single.WorkType, "WorkType = %q, want single", single.WorkType)

	require.Equal(t, "/video/7", single.Route, "Route = %q", single.Route)

	require.Equal(t, "标题", single.Title, "candidate not trimmed: %#v", single)
	require.Equal(t, "科技", single.Partition, "candidate not trimmed: %#v", single)
	require.Equal(t, "http://cover", single.Cover, "candidate not trimmed: %#v", single)

	require.Len(t, []rune(single.Description), 243, "Description len = %d, want 240 runes plus ellipsis", len([]rune(single.Description)))

	require.Len(t, single.Tags, 2, "Tags = %#v", single.Tags)
	require.Equal(t, "go", single.Tags[0], "Tags = %#v", single.Tags)
	require.Equal(t, testRust, single.Tags[1], "Tags = %#v", single.Tags)

	require.Nil(t, single.PartSummaries, "single work should not carry part summaries: %#v", single.PartSummaries)

	multi := NewPromptVideoCandidate(VideoCandidateInput{
		VideoID: 9,
		Title:   "多P",
		Parts:   []VideoPartInput{{Title: "P1"}, {Title: " P2 "}},
	})

	require.Equal(t, "multi_part", multi.WorkType, "multi part candidate = %#v", multi)
	require.Len(t, multi.PartSummaries, 2, "multi part candidate = %#v", multi)

	require.Equal(t, 2, multi.PartSummaries[1].P, "part summary = %#v", multi.PartSummaries[1])
	require.Equal(t, "P2", multi.PartSummaries[1].Title, "part summary = %#v", multi.PartSummaries[1])
	require.Equal(t, "/video/9/2", multi.PartSummaries[1].Route, "part summary = %#v", multi.PartSummaries[1])
}

func TestBuildPromptVideoContext(t *testing.T) {
	t.Parallel()

	empty, err := BuildPromptVideoContext(8, PromptVideoCandidateSet{})
	require.NoError(t, err, "BuildPromptVideoContext error: %v", err)

	require.Contains(t, empty, "当前候选来源：没有候选作品。", "empty context missing default source line: %q", empty)

	require.Contains(t, empty, "本轮作品候选上限：8", "empty context missing limit line: %q", empty)

	require.Contains(t, empty, "最多返回 5 个链接结果", "empty context missing link cap: %q", empty)

	modes := map[string]string{
		RetrievalModeVector:  "语义向量召回结果",
		RetrievalModeKeyword: "关键词命中结果",
		RetrievalModeLatest:  "按最新发布时间回退",
	}

	for mode, marker := range modes {
		got, err := BuildPromptVideoContext(8, PromptVideoCandidateSet{
			RetrievalMode: mode,
			QueryKeywords: []string{"go", "并发"},
			Candidates:    []PromptVideoCandidate{{VideoID: 1, Title: "T", Route: "/video/1", WorkType: testSingle}},
		})
		require.NoError(t, err, "BuildPromptVideoContext(%s) error: %v", mode, err)

		require.Contains(t, got, marker, "context for %s missing %q: %q", mode, marker, got)

		require.Contains(t, got, "第一轮 AI 提炼出的搜索关键词：go、并发", "context for %s missing keywords: %q", mode, got)

		require.Contains(t, got, `"video_id": 1`, "context for %s missing candidate JSON: %q", mode, got)
	}
}

func TestPromptVideoCandidateJSONShape(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(PromptVideoCandidate{VideoID: 3, Title: "T", Route: "/video/3", WorkType: testSingle})
	require.NoError(t, err, "marshal candidate: %v", err)

	want := `{"video_id":3,"title":"T","route":"/video/3","work_type":"single"}`
	require.Equal(t, want, string(raw), "candidate JSON = %s, want %s", raw, want)
}

func TestEnsurePositiveLimit(t *testing.T) {
	t.Parallel()

	{
		got := EnsurePositiveLimit(0)
		require.Equal(t, DefaultCandidateLimit, got, "EnsurePositiveLimit(0) = %d", got)
	}

	{
		got := EnsurePositiveLimit(-3)
		require.Equal(t, DefaultCandidateLimit, got, "EnsurePositiveLimit(-3) = %d", got)
	}

	{
		got := EnsurePositiveLimit(4)
		require.Equal(t, 4, got, "EnsurePositiveLimit(4) = %d", got)
	}
}
