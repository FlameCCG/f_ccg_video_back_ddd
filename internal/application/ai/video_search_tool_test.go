package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"f_ccg_video_back_ddd/internal/config"
	aidomain "f_ccg_video_back_ddd/internal/domain/ai"
	"github.com/cloudwego/eino/components/tool"
	"go.uber.org/zap"
)

type queryOnlyPromptQuery struct {
	promptQueryFake
	keywords []string
}

func (q *queryOnlyPromptQuery) KeywordSearchIDs(_ context.Context, keywords []string, _ int) ([]uint, error) {
	q.keywords = append([]string(nil), keywords...)
	for _, keyword := range keywords {
		if strings.Contains(q.inputs[0].Title, keyword) {
			return []uint{q.inputs[0].VideoID}, nil
		}
	}
	return nil, nil
}

func TestSearchToolSQLFallbackForLongQueries(t *testing.T) {
	t.Parallel()
	const title = "Go语言并发编程与复杂网络服务开发完整系列教程：从协程和Channel到生产环境部署实战"
	for _, tc := range []struct {
		name        string
		vectors     VideoVectorSearch
		keywords    []string
		wantKeyword string
	}{
		{name: "vector search disabled"},
		{name: "vector search unavailable", vectors: &promptVectorFake{err: errors.New("embedding unavailable")}},
		{name: "vector search has no matches", vectors: &promptVectorFake{}},
		{name: "valid keyword takes precedence", keywords: []string{"Go"}, wantKeyword: "Go"},
		{name: "invalid keywords fall back to query", keywords: []string{strings.Repeat("长", 33)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			query := &queryOnlyPromptQuery{promptQueryFake: promptQueryFake{
				available: true, inputs: []aidomain.VideoCandidateInput{{VideoID: 7, Title: title}},
			}}
			search := &videoSearchTool{service: &Service{prompts: query, vectors: tc.vectors, logger: zap.NewNop()}}
			arguments, _ := json.Marshal(videoSearchArguments{Query: title, Keywords: tc.keywords})
			raw, err := search.InvokableRun(t.Context(), string(arguments))
			if err != nil {
				t.Fatal(err)
			}
			var result videoSearchResult
			if err := json.Unmarshal([]byte(raw), &result); err != nil {
				t.Fatal(err)
			}
			wantKeyword := tc.wantKeyword
			if wantKeyword == "" {
				wantKeyword = title
			}
			if len(query.keywords) != 1 || query.keywords[0] != wantKeyword || result.RetrievalMode != aidomain.RetrievalModeKeyword || result.ResultCount != 1 || result.Candidates[0].VideoID != 7 {
				t.Fatalf("query-only SQL fallback lost the title: keywords=%v result=%+v", query.keywords, result)
			}
		})
	}
}

func TestSearchToolUsesRewrittenQueryAndCurrentImageWithoutAnalysisCall(t *testing.T) {
	t.Parallel()
	vectors := &promptVectorFake{ids: []uint{7}}
	query := &promptQueryFake{available: true, inputs: []aidomain.VideoCandidateInput{{VideoID: 7, Title: "Go并发", Parts: []aidomain.VideoPartInput{{Title: "协程"}, {Title: "Channel"}}}}}
	service := &Service{prompts: query, vectors: vectors, logger: zap.NewNop()}
	cfg := &config.AI{VectorTopK: 3}
	tools := service.conversationTools(cfg, conversation{LatestUserText: "再找几个这样的", LatestUserImages: []string{"data:image/png;base64,IMAGE_DATA", "https://example.com/ignored.png"}})
	cfg.VectorTopK = 99
	if len(tools) != 1 {
		t.Fatal("search tool not registered")
	}
	result, err := tools[0].(tool.InvokableTool).InvokableRun(t.Context(), `{"query":"Go并发入门教程","keywords":["Go","并发"]}`)
	if err != nil {
		t.Fatal(err)
	}
	if vectors.last.Text != "Go并发入门教程" || vectors.last.ImageURL != "data:image/png;base64,IMAGE_DATA" || vectors.limit != 3 {
		t.Fatalf("query=%+v limit=%d", vectors.last, vectors.limit)
	}
	var payload videoSearchResult
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ResultCount != 1 || payload.Candidates[0].Route != "/video/7" || payload.Candidates[0].PartSummaries[1].Route != "/video/7/2" {
		t.Fatalf("tool payload=%+v", payload)
	}
	if strings.Contains(result, "IMAGE_DATA") {
		t.Fatal("request image leaked in tool result")
	}
}

func TestSearchToolAllowsQueryWithoutSeparateAnalysisKeywords(t *testing.T) {
	t.Parallel()
	vectors := &promptVectorFake{ids: []uint{7}}
	service := &Service{prompts: &promptQueryFake{available: true, inputs: []aidomain.VideoCandidateInput{{VideoID: 7, Title: "Go"}}}, vectors: vectors, logger: zap.NewNop()}
	search := &videoSearchTool{service: service}
	_, err := search.InvokableRun(t.Context(), `{"query":"帮我找系统讲解Go语言并发编程和复杂网络服务开发的完整系列教程"}`)
	if err != nil || vectors.last.Text == "" {
		t.Fatalf("query without extracted keywords was skipped: %v %+v", err, vectors.last)
	}
}

func TestSearchToolValidatesArgumentsAndBoundsResults(t *testing.T) {
	t.Parallel()
	service := &Service{prompts: &promptQueryFake{available: true, latest: []uint{7, 8}, inputs: []aidomain.VideoCandidateInput{{VideoID: 7, Title: strings.Repeat("x", 3000)}, {VideoID: 8, Title: "Short"}}}, logger: zap.NewNop()}
	search := &videoSearchTool{service: service, cfg: config.AI{VectorTopK: 1, ContextMaxChars: 4000}}
	for _, arguments := range []string{`{`, `{"query":""}`, `{"query":"` + strings.Repeat("长", 1001) + `"}`} {
		if _, err := search.InvokableRun(t.Context(), arguments); err == nil {
			t.Fatalf("invalid arguments accepted: %s", arguments[:min(len(arguments), 50)])
		}
	}
	result, err := search.InvokableRun(t.Context(), `{"query":"Go"}`)
	if err != nil {
		t.Fatal(err)
	}
	var payload videoSearchResult
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ResultCount > 1 || len([]rune(result)) > 2000 {
		t.Fatalf("tool result exceeded its budget: count=%d chars=%d", payload.ResultCount, len([]rune(result)))
	}
}
