package ai

import (
	"context"
	"encoding/json"
	"errors"
	"f_ccg_video_back_ddd/internal/config"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	domain "f_ccg_video_back_ddd/internal/domain/ai"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type promptTextFake struct {
	text  string
	err   error
	last  TextInvocation
	calls int
}

func (f *promptTextFake) InvokeText(_ context.Context, invocation TextInvocation) (string, error) {
	f.last = invocation
	f.calls++

	return f.text, f.err
}

func TestBuildResponsesSystemPromptSkipsAnalysisWithoutVideoSource(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		prompts PromptVideoQuery
	}{
		{name: "not configured"},
		{name: "unavailable", prompts: &promptQueryFake{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			text := &promptTextFake{err: errors.New("model must not be called")}
			s := &Service{text: text, prompts: tc.prompts, logger: zap.NewNop()}

			prompt, err := s.buildResponsesSystemPrompt(context.Background(), &config.AI{SystemPrompt: "服务端提示"}, RuntimeConfig{}, conversation{
				Messages: []*schema.Message{schema.UserMessage("你好")},
			}, "test-model")
			require.NoError(t, err, "prompt=%q err=%v modelCalls=%d", prompt, err, text.calls)
			require.Equal(t, "服务端提示", prompt, "prompt=%q err=%v modelCalls=%d", prompt, err, text.calls)
			require.Equal(t, 0, text.calls, "prompt=%q err=%v modelCalls=%d", prompt, err, text.calls)
		})
	}
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
		{
			got := extractJSONObject(input)
			require.Equal(t, want, got, "extractJSONObject(%q) = %q, want %q", input, got, want)
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
	require.NoError(t, err, "parseConversation: %v", err)

	runtime := RuntimeConfig{
		ChatModelBaseURL: testHTTPSOldChatExampleV1,
		ChatModelAPIKey:  testOldChatKey,
		DoubaoAPIKey:     testOldDoubaoKey,
		TimeoutSec:       11,
	}

	analysis, err := service.analyzeVideoQuery(
		context.Background(),
		&config.AI{},
		runtime,
		"chat-model",
		conv,
	)
	require.NoError(t, err, "analyzeVideoQuery: %v", err)

	require.Equal(t, "find_video", analysis.Intent, "analysis = %+v", analysis)
	require.True(t, reflect.DeepEqual(analysis.Keywords, []string{"Go"}), "analysis = %+v", analysis)

	require.Equal(t, domain.AnalysisMaxOutputTokens, text.last.MaxTokens, "invocation = %+v", text.last)
	require.Len(t, text.last.Messages, 2, "invocation = %+v", text.last)

	require.Equal(t, text.last.Runtime, runtime, "runtime = %+v, want %+v", text.last.Runtime, runtime)

	transcript := text.last.Messages[1].Content
	require.NotContains(t, transcript, "SECRET", "transcript = %q", transcript)
	require.Contains(t, transcript, "用户本轮还上传了图片", "transcript = %q", transcript)
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
	require.True(t, reflect.DeepEqual(ids, []uint{9}), "ids=%v mode=%q", ids, mode)
	require.Equal(t, domain.RetrievalModeImageVector, mode, "ids=%v mode=%q", ids, mode)

	want := VideoVectorQuery{
		Text:     "帮我找一辆红色跑车的视频",
		ImageURL: "data:image/png;base64,ONE",
	}
	require.Equal(t, want, vectors.last, "query = %+v, want %+v; limit=%d", vectors.last, want, vectors.limit)
	require.Equal(t, 4, vectors.limit, "query = %+v, want %+v; limit=%d", vectors.last, want, vectors.limit)
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
	require.NoError(t, err, "loadPromptVideoCandidates: %v", err)

	require.Equal(t, domain.RetrievalModeLatest, set.RetrievalMode, "candidate set = %+v", set)
	require.Len(t, set.Candidates, 1, "candidate set = %+v", set)

	require.True(t, reflect.DeepEqual(prompts.loadedIDs, []uint{7}), "loaded IDs = %v", prompts.loadedIDs)
}
