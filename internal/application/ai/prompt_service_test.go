package ai

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"f_ccg_video_back_ddd/internal/config"
	domain "f_ccg_video_back_ddd/internal/domain/ai"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type promptTextFake struct {
	text string
	err  error
	last TextInvocation
}

func (f *promptTextFake) InvokeText(_ context.Context, invocation TextInvocation) (string, error) {
	f.last = invocation

	return f.text, f.err
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
	inputs     []domain.VideoCandidateInput
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
) ([]domain.VideoCandidateInput, error) {
	f.loadedIDs = append([]uint(nil), ids...)

	return f.inputs, f.loadErr
}

func TestExtractJSONObjectStripsModelNoise(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		`{"intent":"find_video","keywords":["go"]}`:                 `{"intent":"find_video","keywords":["go"]}`,
		"```json\n{\"intent\":\"chat\",\"keywords\":[]}\n```":       `{"intent":"chat","keywords":[]}`,
		"分析如下：{\"intent\":\"find_video\",\"keywords\":[\"爬虫\"]} 完毕": `{"intent":"find_video","keywords":["爬虫"]}`,
	}
	for input, want := range tests {
		if got := extractJSONObject(input); got != want {
			t.Fatalf("extractJSONObject(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAnalyzeVideoQueryUsesBoundedTranscriptWithoutBase64(t *testing.T) {
	t.Parallel()

	text := &promptTextFake{text: `{"intent":"find_video","keywords":[" Go ","GO"]}`}
	service := &Service{text: text, logger: zap.NewNop()}
	conv, err := parseConversation(json.RawMessage(`[
		{"role":"assistant","content":"上轮回答"},
		{"role":"user","content":[
			{"type":"input_text","text":"帮我找这个教程"},
			{"type":"input_image","image_url":"data:image/jpeg;base64,SECRET"}
		]}
	]`))
	if err != nil {
		t.Fatalf("parseConversation: %v", err)
	}

	runtime := RuntimeConfig{
		ChatModelBaseURL: "https://old-chat.example/v1",
		ChatModelAPIKey:  "old-chat-key",
		DoubaoAPIKey:     "old-doubao-key",
		TimeoutSec:       11,
	}
	analysis, err := service.analyzeVideoQuery(
		context.Background(),
		&config.AI{},
		runtime,
		"chat-model",
		conv,
	)
	if err != nil {
		t.Fatalf("analyzeVideoQuery: %v", err)
	}
	if analysis.Intent != "find_video" || !reflect.DeepEqual(analysis.Keywords, []string{"Go"}) {
		t.Fatalf("analysis = %+v", analysis)
	}
	if text.last.MaxTokens != domain.AnalysisMaxOutputTokens || len(text.last.Messages) != 2 {
		t.Fatalf("invocation = %+v", text.last)
	}
	if text.last.Runtime != runtime {
		t.Fatalf("runtime = %+v, want %+v", text.last.Runtime, runtime)
	}
	transcript := text.last.Messages[1].Content
	if strings.Contains(transcript, "SECRET") || !strings.Contains(transcript, "用户本轮还上传了图片") {
		t.Fatalf("transcript = %q", transcript)
	}
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
	if !reflect.DeepEqual(ids, []uint{9}) || mode != domain.RetrievalModeImageVector {
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
		inputs: []domain.VideoCandidateInput{{
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
	if set.RetrievalMode != domain.RetrievalModeLatest || len(set.Candidates) != 1 {
		t.Fatalf("candidate set = %+v", set)
	}
	if !reflect.DeepEqual(prompts.loadedIDs, []uint{7}) {
		t.Fatalf("loaded IDs = %v", prompts.loadedIDs)
	}
}
