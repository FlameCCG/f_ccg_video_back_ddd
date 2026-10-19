package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"f_ccg_video_back_ddd/internal/config"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
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
	if err == nil {
		t.Fatal("prepareResponsesRequest() expected an error")
	}
	if got, want := err.Error(), "不支持的思考强度: unsupported"; got != want {
		t.Fatalf("prepareResponsesRequest() error = %q, want %q", got, want)
	}
}

func TestCreateResponseKeepsRuntimeSnapshotAcrossHotReload(t *testing.T) {
	oldAI := &config.AI{
		ChatModelBaseURL:  " https://old-chat.example/v1/ ",
		ChatModelAPIKey:   " old-chat-key ",
		DoubaoModelAPIKey: " old-doubao-key ",
		ChatModel:         "old-chat-model",
		TimeoutSec:        17,
	}
	provider := infraconfig.NewProvider(&config.Config{AI: oldAI})
	text := &snapshotTextGenerator{
		provider: provider,
		next: &config.Config{AI: &config.AI{
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
	if err != nil {
		t.Fatalf("CreateResponse() error = %v", err)
	}
	if len(text.invocations) != 2 {
		t.Fatalf("invocation count = %d, want 2", len(text.invocations))
	}

	wantRuntime := RuntimeConfig{
		ChatModelBaseURL: "https://old-chat.example/v1",
		ChatModelAPIKey:  "old-chat-key",
		DoubaoAPIKey:     "old-doubao-key",
		TimeoutSec:       17,
	}
	for i, invocation := range text.invocations {
		if invocation.Runtime != wantRuntime {
			t.Fatalf("invocation[%d].Runtime = %+v, want %+v", i, invocation.Runtime, wantRuntime)
		}
		if invocation.Model != "old-chat-model" {
			t.Fatalf("invocation[%d].Model = %q, want old-chat-model", i, invocation.Model)
		}
	}
}

func TestGenerateImageKeepsRuntimeSnapshotForSupplementRequests(t *testing.T) {
	oldAI := &config.AI{
		ChatModelBaseURL:  "https://old-chat.example/v1",
		ChatModelAPIKey:   "old-chat-key",
		DoubaoModelAPIKey: "old-doubao-key",
		ImageModel:        "old-image-model",
		TimeoutSec:        23,
	}
	provider := infraconfig.NewProvider(&config.Config{AI: oldAI})
	images := &snapshotImageGenerator{
		provider: provider,
		next: &config.Config{AI: &config.AI{
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
	if err != nil {
		t.Fatalf("GenerateImage() error = %v", err)
	}
	if len(images.invocations) != 2 {
		t.Fatalf("invocation count = %d, want 2", len(images.invocations))
	}

	wantRuntime := RuntimeConfig{
		ChatModelBaseURL: "https://old-chat.example/v1",
		ChatModelAPIKey:  "old-chat-key",
		DoubaoAPIKey:     "old-doubao-key",
		TimeoutSec:       23,
	}
	for i, invocation := range images.invocations {
		if invocation.Runtime != wantRuntime {
			t.Fatalf("invocation[%d].Runtime = %+v, want %+v", i, invocation.Runtime, wantRuntime)
		}
		if invocation.Model != "old-image-model" {
			t.Fatalf("invocation[%d].Model = %q, want old-image-model", i, invocation.Model)
		}
	}
	root, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want object", result)
	}
	if data, dataOK := root["data"].([]any); !dataOK || len(data) != wantImages {
		t.Fatalf("result = %#v, want %d images", result, wantImages)
	}
}

func TestGetVideoPassesCapturedRuntimeConfig(t *testing.T) {
	provider := infraconfig.NewProvider(&config.Config{AI: &config.AI{
		ChatModelBaseURL:  "https://old-chat.example/v1",
		ChatModelAPIKey:   "old-chat-key",
		DoubaoModelAPIKey: "old-doubao-key",
		TimeoutSec:        31,
	}})
	videos := &snapshotVideoClient{
		provider: provider,
		next: &config.Config{AI: &config.AI{
			DoubaoModelAPIKey: "new-doubao-key",
			TimeoutSec:        99,
		}},
	}
	service := NewService(provider, nil, nil, videos, nil, nil, nil, zap.NewNop())

	if _, err := service.GetVideo(context.Background(), "task-old"); err != nil {
		t.Fatalf("GetVideo() error = %v", err)
	}
	wantRuntime := RuntimeConfig{
		ChatModelBaseURL: "https://old-chat.example/v1",
		ChatModelAPIKey:  "old-chat-key",
		DoubaoAPIKey:     "old-doubao-key",
		TimeoutSec:       31,
	}
	if videos.runtime != wantRuntime || videos.requestID != "task-old" {
		t.Fatalf("Get() runtime=%+v requestID=%q, want %+v/task-old", videos.runtime, videos.requestID, wantRuntime)
	}
}

func TestOpenAssetStreamPassesCapturedRuntimeConfig(t *testing.T) {
	provider := infraconfig.NewProvider(&config.Config{AI: &config.AI{
		ChatModelBaseURL:  "https://old-chat.example/v1",
		ChatModelAPIKey:   "old-chat-key",
		DoubaoModelAPIKey: "old-doubao-key",
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
	if err != nil {
		t.Fatalf("OpenAssetStream() error = %v", err)
	}
	_ = response.Body.Close()

	wantRuntime := RuntimeConfig{
		ChatModelBaseURL: "https://old-chat.example/v1",
		ChatModelAPIKey:  "old-chat-key",
		DoubaoAPIKey:     "old-doubao-key",
		TimeoutSec:       37,
	}
	if assets.runtime != wantRuntime || assets.assetURL != "https://ark-content.volces.com/object" {
		t.Fatalf("Open() runtime=%+v assetURL=%q, want %+v", assets.runtime, assets.assetURL, wantRuntime)
	}
}
