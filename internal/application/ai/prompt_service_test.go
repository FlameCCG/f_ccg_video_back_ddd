package ai

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"f_ccg_video_back_ddd/internal/config"
	aidomain "f_ccg_video_back_ddd/internal/domain/ai"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type promptTextFake struct {
	text  string
	err   error
	last  TextInvocation
	calls int
}

func TestPromptCandidateLimitKeepsRecallAndPromptInAgreement(t *testing.T) {
	for _, tc := range []struct{ configured, want int }{{0, 8}, {-1, 8}, {3, 3}, {25, 25}} {
		t.Run(strconv.Itoa(tc.configured), func(t *testing.T) {
			vectors := &promptVectorFake{ids: []uint{7}}
			service := &Service{
				vectors: vectors,
				prompts: &promptQueryFake{available: true, inputs: []aidomain.VideoCandidateInput{{VideoID: 7, Title: "Go"}}},
				logger:  zap.NewNop(),
			}
			search := &videoSearchTool{service: service, cfg: config.AI{VectorTopK: tc.configured}}
			raw, err := search.InvokableRun(t.Context(), `{"query":"找 Go 视频","keywords":["Go"]}`)
			if err != nil {
				t.Fatal(err)
			}
			var result videoSearchResult
			if err := json.Unmarshal([]byte(raw), &result); err != nil {
				t.Fatal(err)
			}
			if vectors.limit != tc.want || result.ResultCount != 1 {
				t.Fatalf("recall limit=%d want=%d result=%+v", vectors.limit, tc.want, result)
			}
		})
	}
}

func (f *promptTextFake) InvokeText(_ context.Context, invocation TextInvocation) (TextResult, error) {
	f.last = invocation
	f.calls++

	return TextResult{Text: f.text, ResponseMetadata: ResponseMetadata{FinishReason: "end_turn"}}, f.err
}

func (f *promptTextFake) OpenStream(
	context.Context,
	TextInvocation,
) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("not implemented")
}

type promptVectorFake struct {
	ids   []uint
	err   error
	last  VideoVectorQuery
	limit int
}

func (f *promptVectorFake) SearchVideoVectorIDsMultiModal(
	_ context.Context,
	query VideoVectorQuery,
	limit int,
) ([]uint, error) {
	f.last = query
	f.limit = limit

	return f.ids, f.err
}

type promptQueryFake struct {
	available  bool
	keyword    []uint
	latest     []uint
	inputs     []aidomain.VideoCandidateInput
	keywordErr error
	latestErr  error
	loadErr    error
	loadedIDs  []uint
}

func (f *promptQueryFake) Available() bool { return f.available }

func (f *promptQueryFake) KeywordSearchIDs(
	context.Context,
	[]string,
	int,
) ([]uint, error) {
	return f.keyword, f.keywordErr
}

func (f *promptQueryFake) LatestPublishedIDs(context.Context, int) ([]uint, error) {
	return f.latest, f.latestErr
}

func (f *promptQueryFake) Load(
	_ context.Context,
	ids []uint,
) ([]aidomain.VideoCandidateInput, error) {
	f.loadedIDs = append([]uint(nil), ids...)

	return f.inputs, f.loadErr
}

func TestSearchPromptVideoIDsUsesOriginalTextAndFirstImage(t *testing.T) {
	t.Parallel()

	vectors := &promptVectorFake{ids: []uint{9}}
	service := &Service{
		vectors: vectors,
		prompts: &promptQueryFake{available: true},
		logger:  zap.NewNop(),
	}
	conv := conversation{
		LatestUserText:   "  帮我找一辆红色跑车的视频  ",
		LatestUserImages: []string{"data:image/png;base64,ONE", "data:image/png;base64,TWO"},
	}

	ids, mode := service.searchPromptVideoIDs(context.Background(), conv, []string{"跑车"}, 4)
	if !reflect.DeepEqual(ids, []uint{9}) || mode != aidomain.RetrievalModeImageVector {
		t.Fatalf("ids=%v mode=%q", ids, mode)
	}
	want := VideoVectorQuery{
		Text:     "帮我找一辆红色跑车的视频",
		ImageURL: "data:image/png;base64,ONE",
	}
	if vectors.last != want || vectors.limit != 4 {
		t.Fatalf("query = %+v, want %+v; limit=%d", vectors.last, want, vectors.limit)
	}
}

func TestLoadPromptVideoCandidatesFallsBackToLatestAfterRecallErrors(t *testing.T) {
	t.Parallel()

	prompts := &promptQueryFake{
		available:  true,
		keywordErr: errors.New("mysql recall failed"),
		latest:     []uint{7},
		inputs: []aidomain.VideoCandidateInput{{
			VideoID: 7,
			Title:   "最新作品",
		}},
	}
	service := &Service{
		vectors: &promptVectorFake{err: errors.New("vector failed")},
		prompts: prompts,
		logger:  zap.NewNop(),
	}
	conv := conversation{LatestUserText: "找 Go 教程"}

	set, err := service.loadPromptVideoCandidates(context.Background(), conv, []string{"Go"}, 3)
	if err != nil {
		t.Fatalf("loadPromptVideoCandidates: %v", err)
	}
	if set.RetrievalMode != aidomain.RetrievalModeLatest || len(set.Candidates) != 1 {
		t.Fatalf("candidate set = %+v", set)
	}
	if !reflect.DeepEqual(prompts.loadedIDs, []uint{7}) {
		t.Fatalf("loaded IDs = %v", prompts.loadedIDs)
	}
}
