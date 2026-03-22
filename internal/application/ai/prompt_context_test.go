package ai

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	aidomain "f_ccg_video_back_ddd/internal/domain/ai"
)

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

			if got := (promptVideoContext{}).videoRoute(tt.videoID, tt.p); got != tt.want {
				t.Fatalf("BuildVideoRoute(%d, %d) = %q, want %q", tt.videoID, tt.p, got, tt.want)
			}
		})
	}
}

func TestBuildPromptVideoContext(t *testing.T) {
	t.Parallel()

	empty, err := (promptVideoContext{}).Build(aidomain.PromptVideoCandidateSet{}, 8)
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
		aidomain.RetrievalModeVector:  "语义向量召回结果",
		aidomain.RetrievalModeKeyword: "关键词命中结果",
		aidomain.RetrievalModeLatest:  "按最新发布时间回退",
	}

	for mode, marker := range modes {
		got, err := (promptVideoContext{}).Build(aidomain.PromptVideoCandidateSet{
			RetrievalMode: mode,
			QueryKeywords: []string{"go", "并发"},
			Candidates:    []aidomain.PromptVideoCandidate{{VideoID: 1, Title: "T", WorkType: "single"}},
		}, 8)
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

	raw, err := json.Marshal((promptVideoContext{}).candidate(aidomain.PromptVideoCandidate{VideoID: 3, Title: "T", WorkType: "single"}))
	if err != nil {
		t.Fatalf("marshal candidate: %v", err)
	}

	want := `{"video_id":3,"title":"T","route":"/video/3","work_type":"single"}`
	if string(raw) != want {
		t.Fatalf("candidate JSON = %s, want %s", raw, want)
	}
}

func TestPromptVideoContextPreservesLegacyOutput(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/prompt_video_context.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name     string
		Mode     string
		Keywords []string
		Inputs   []aidomain.VideoCandidateInput
		Limit    int
		Want     string
	}
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			set := aidomain.PromptVideoCandidateSet{RetrievalMode: fixture.Mode, QueryKeywords: fixture.Keywords}
			for _, input := range fixture.Inputs {
				set.Candidates = append(set.Candidates, aidomain.NewPromptVideoCandidate(input))
			}
			got, err := (promptVideoContext{}).Build(set, fixture.Limit)
			if err != nil {
				t.Fatal(err)
			}
			if got != fixture.Want {
				t.Fatalf("prompt changed:\n%q\nwant:\n%q", got, fixture.Want)
			}
		})
	}
}
