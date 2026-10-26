package ai

import (
	"context"
	"encoding/json"
	"f_ccg_video_back_ddd/internal/config"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

const (
	testHTTPSOldChatExampleV1 = "https://old-chat.example/v1"
	testOldChatKey            = "old-chat-key"
	testOldDoubaoKey          = "old-doubao-key"
)

type snapshotTextGenerator struct {
	provider    *infraconfig.Provider
	next        *config.Config
	invocations []TextInvocation
}

func (g *snapshotTextGenerator) InvokeText(
	_ context.Context,
	invocation TextInvocation,
) (string, error) {
	g.invocations = append(g.invocations, invocation)
	if len(g.invocations) == 1 {
		g.provider.Set(g.next)

		return `{"intent":"chat","keywords":[]}`, nil
	}

	return "snapshot answer", nil
}

func (*snapshotTextGenerator) OpenStream(
	context.Context,
	TextInvocation,
) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

type snapshotImageGenerator struct {
	provider    *infraconfig.Provider
	next        *config.Config
	invocations []ImageInvocation
}

func (g *snapshotImageGenerator) Generate(
	_ context.Context,
	invocation ImageInvocation,
) (any, error) {
	g.invocations = append(g.invocations, invocation)
	if len(g.invocations) == 1 {
		g.provider.Set(g.next)
	}

	return map[string]any{
		"data": []any{map[string]any{"url": "https://result.example/image.png"}},
	}, nil
}

type snapshotVideoClient struct {
	provider  *infraconfig.Provider
	next      *config.Config
	runtime   RuntimeConfig
	requestID string
}

func (*snapshotVideoClient) Create(context.Context, VideoInvocation) (any, error) {
	return nil, nil
}

func (c *snapshotVideoClient) Get(
	_ context.Context,
	runtime RuntimeConfig,
	requestID string,
) (any, error) {
	c.provider.Set(c.next)
	c.runtime = runtime
	c.requestID = requestID

	return map[string]any{"requestId": requestID}, nil
}

type snapshotAssetFetcher struct {
	provider *infraconfig.Provider
	next     *config.Config
	runtime  RuntimeConfig
	assetURL string
}

func (f *snapshotAssetFetcher) Open(
	_ context.Context,
	runtime RuntimeConfig,
	assetURL string,
	_ http.Header,
) (*http.Response, error) {
	f.provider.Set(f.next)
	f.runtime = runtime
	f.assetURL = assetURL

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}, nil
}

func TestPrepareResponsesRequestValidatesThinkingBeforeConversation(t *testing.T) {
	t.Parallel()

	cfg := &config.AI{
		ChatModelAPIKey: "test-key",
		ChatModel:       "test-chat",
		ThinkingEnabled: true,
		ThinkingEffort:  "high",
		ThinkingEfforts: []config.AIModel{
			{Label: "High", Value: "high"},
		},
	}
	service := &Service{
		logger: zap.NewNop(),
	}

	_, _, err := service.prepareResponsesRequest(context.Background(), cfg, RuntimeConfig{}, ResponsesRequest{
		Input:          json.RawMessage(`{`),
		ThinkingEffort: "unsupported",
	})
	require.Error(t, err, "prepareResponsesRequest() expected an error")

	{
		got, want := err.Error(), "不支持的思考强度: unsupported"
		require.Equal(t, want, got, "prepareResponsesRequest() error = %q, want %q", got, want)
	}
}

func TestCreateResponseKeepsRuntimeSnapshotAcrossHotReload(t *testing.T) {
	t.Parallel()

	oldAI := &config.AI{ // #nosec G101 -- Deliberately fake credentials used only by this isolated test fixture.
		ChatModelBaseURL:  " https://old-chat.example/v1/ ",
		ChatModelAPIKey:   " old-chat-key ",
		DoubaoModelAPIKey: " old-doubao-key ",
		ChatModel:         "old-chat-model",
		TimeoutSec:        17,
	}
	provider := infraconfig.NewProvider(&config.Config{AI: oldAI})
	text := &snapshotTextGenerator{
		provider: provider,
		next: &config.Config{AI: &config.AI{ // #nosec G101 -- Deliberately fake credentials used only by this isolated test fixture.
			ChatModelBaseURL:  "https://new-chat.example/v1",
			ChatModelAPIKey:   "new-chat-key",
			DoubaoModelAPIKey: "new-doubao-key",
			ChatModel:         "new-chat-model",
			TimeoutSec:        99,
		}},
	}
	service := NewService(provider, text, nil, nil, nil, &promptQueryFake{available: true}, nil, zap.NewNop())

	_, err := service.CreateResponse(context.Background(), ResponsesRequest{
		Input: json.RawMessage(`[{"role":"user","content":"hello"}]`),
	})
	require.NoError(t, err, "CreateResponse() error = %v", err)

	require.Len(t, text.invocations, 2, "invocation count = %d, want 2", len(text.invocations))

	wantRuntime := RuntimeConfig{
		ChatModelBaseURL: testHTTPSOldChatExampleV1,
		ChatModelAPIKey:  testOldChatKey,
		DoubaoAPIKey:     testOldDoubaoKey,
		TimeoutSec:       17,
	}
	for i, invocation := range text.invocations {
		require.Equal(t, wantRuntime, invocation.Runtime, "invocation[%d].Runtime = %+v, want %+v", i, invocation.Runtime, wantRuntime)

		require.Equal(t, "old-chat-model", invocation.Model, "invocation[%d].Model = %q, want old-chat-model", i, invocation.Model)
	}
}

func TestGenerateImageKeepsRuntimeSnapshotForSupplementRequests(t *testing.T) {
	t.Parallel()

	oldAI := &config.AI{
		ChatModelBaseURL:  testHTTPSOldChatExampleV1,
		ChatModelAPIKey:   testOldChatKey,
		DoubaoModelAPIKey: testOldDoubaoKey,
		ImageModel:        "old-image-model",
		TimeoutSec:        23,
	}
	provider := infraconfig.NewProvider(&config.Config{AI: oldAI})
	images := &snapshotImageGenerator{
		provider: provider,
		next: &config.Config{AI: &config.AI{ // #nosec G101 -- Deliberately fake credentials used only by this isolated test fixture.
			ChatModelBaseURL:  "https://new-chat.example/v1",
			ChatModelAPIKey:   "new-chat-key",
			DoubaoModelAPIKey: "new-doubao-key",
			ImageModel:        "new-image-model",
			TimeoutSec:        99,
		}},
	}
	service := NewService(provider, nil, images, nil, nil, nil, nil, zap.NewNop())
	wantImages := 2

	result, err := service.GenerateImage(context.Background(), ImageGenerationRequest{
		Prompt: "draw a cat",
		N:      &wantImages,
	})
	require.NoError(t, err, "GenerateImage() error = %v", err)

	require.Len(t, images.invocations, 2, "invocation count = %d, want 2", len(images.invocations))

	wantRuntime := RuntimeConfig{
		ChatModelBaseURL: testHTTPSOldChatExampleV1,
		ChatModelAPIKey:  testOldChatKey,
		DoubaoAPIKey:     testOldDoubaoKey,
		TimeoutSec:       23,
	}
	for i, invocation := range images.invocations {
		require.Equal(t, wantRuntime, invocation.Runtime, "invocation[%d].Runtime = %+v, want %+v", i, invocation.Runtime, wantRuntime)

		require.Equal(t, "old-image-model", invocation.Model, "invocation[%d].Model = %q, want old-image-model", i, invocation.Model)
	}

	root, ok := result.(map[string]any)
	require.True(t, ok, "result = %#v, want object", result)

	{
		data, dataOK := root["data"].([]any)
		require.True(t, dataOK, "result = %#v, want %d images", result, wantImages)
		require.Len(t, data, wantImages, "result = %#v, want %d images", result, wantImages)
	}
}

func TestGetVideoPassesCapturedRuntimeConfig(t *testing.T) {
	t.Parallel()

	provider := infraconfig.NewProvider(&config.Config{AI: &config.AI{
		ChatModelBaseURL:  testHTTPSOldChatExampleV1,
		ChatModelAPIKey:   testOldChatKey,
		DoubaoModelAPIKey: testOldDoubaoKey,
		TimeoutSec:        31,
	}})
	videos := &snapshotVideoClient{
		provider: provider,
		next: &config.Config{AI: &config.AI{ // #nosec G101 -- Deliberately fake credentials used only by this isolated test fixture.
			DoubaoModelAPIKey: "new-doubao-key",
			TimeoutSec:        99,
		}},
	}
	service := NewService(provider, nil, nil, videos, nil, nil, nil, zap.NewNop())

	{
		_, err := service.GetVideo(context.Background(), "task-old")
		require.NoError(t, err, "GetVideo() error = %v", err)
	}

	wantRuntime := RuntimeConfig{
		ChatModelBaseURL: testHTTPSOldChatExampleV1,
		ChatModelAPIKey:  testOldChatKey,
		DoubaoAPIKey:     testOldDoubaoKey,
		TimeoutSec:       31,
	}
	require.Equal(t, wantRuntime, videos.runtime, "Get() runtime=%+v requestID=%q, want %+v/task-old", videos.runtime, videos.requestID, wantRuntime)
	require.Equal(t, "task-old", videos.requestID, "Get() runtime=%+v requestID=%q, want %+v/task-old", videos.runtime, videos.requestID, wantRuntime)
}

func TestOpenAssetStreamPassesCapturedRuntimeConfig(t *testing.T) {
	t.Parallel()

	provider := infraconfig.NewProvider(&config.Config{AI: &config.AI{
		ChatModelBaseURL:  testHTTPSOldChatExampleV1,
		ChatModelAPIKey:   testOldChatKey,
		DoubaoModelAPIKey: testOldDoubaoKey,
		TimeoutSec:        37,
	}})
	assets := &snapshotAssetFetcher{
		provider: provider,
		next: &config.Config{AI: &config.AI{
			TimeoutSec: 99,
		}},
	}
	service := NewService(provider, nil, nil, nil, assets, nil, nil, zap.NewNop())

	response, err := service.OpenAssetStream(
		context.Background(),
		"https://ark-content.volces.com/object",
		http.Header{"Range": []string{"bytes=0-9"}},
	)
	require.NoError(t, err, "OpenAssetStream() error = %v", err)

	_ = response.Body.Close()

	wantRuntime := RuntimeConfig{
		ChatModelBaseURL: testHTTPSOldChatExampleV1,
		ChatModelAPIKey:  testOldChatKey,
		DoubaoAPIKey:     testOldDoubaoKey,
		TimeoutSec:       37,
	}
	require.Equal(t, wantRuntime, assets.runtime, "Open() runtime=%+v assetURL=%q, want %+v", assets.runtime, assets.assetURL, wantRuntime)
	require.Equal(t, "https://ark-content.volces.com/object", assets.assetURL, "Open() runtime=%+v assetURL=%q, want %+v", assets.runtime, assets.assetURL, wantRuntime)
}
