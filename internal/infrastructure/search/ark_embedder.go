package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	app "f_ccg_video_back_ddd/internal/application/search"
	domain "f_ccg_video_back_ddd/internal/domain/search"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// embeddingRetryTimes 方舟向量接口的重试次数。
//
// 原未提交代码引用了一个不存在的常量（见 report.md），此处取方舟 SDK 默认重试次数。
const embeddingRetryTimes = 2

// ArkMultimodalEmbedder 直接调用方舟多模态向量接口。
//
// 不复用 eino-ext 的 embedding/ark 组件：它的 EmbedStrings 在多模态分支里只发
// MultiModalEmbeddingInputTypeText，永远不会传图，无法支撑「以图搜视频」。
type ArkMultimodalEmbedder struct {
	client *arkruntime.Client
	model  string
	dims   int
}

// Embed 把文本与图片融合成一个稠密向量。
func (e *ArkMultimodalEmbedder) Embed(ctx context.Context, in domain.EmbedInput) ([]float32, error) {
	if in.IsZero() {
		return nil, errors.New("embedding input is empty")
	}

	inputs := make([]model.MultimodalEmbeddingInput, 0, 2)

	if text := strings.TrimSpace(in.Text); text != "" {
		inputs = append(inputs, model.MultimodalEmbeddingInput{
			Type: model.MultiModalEmbeddingInputTypeText,
			Text: &text,
		})
	}

	if imageURL := strings.TrimSpace(in.ImageURL); imageURL != "" {
		inputs = append(inputs, model.MultimodalEmbeddingInput{
			Type:     model.MultiModalEmbeddingInputTypeImageURL,
			ImageURL: &model.MultimodalEmbeddingImageURL{URL: imageURL},
		})
	}

	encodingFormat := model.EmbeddingEncodingFormatFloat
	req := model.MultiModalEmbeddingRequest{
		Input:          inputs,
		Model:          e.model,
		EncodingFormat: &encodingFormat,
	}

	if e.dims > 0 {
		dims := e.dims
		req.Dimensions = &dims
	}

	resp, err := e.client.CreateMultiModalEmbeddings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("call ark multimodal embeddings failed: %w", err)
	}

	vector := resp.Data.Embedding
	if len(vector) == 0 {
		return nil, errors.New("ark multimodal embeddings returned an empty vector")
	}

	if e.dims > 0 && len(vector) != e.dims {
		return nil, fmt.Errorf(
			"embedding dimension mismatch: configured=%d, got=%d; 请同步修改 embeddingDimensions 或重建向量索引",
			e.dims, len(vector),
		)
	}

	return vector, nil
}

// arkEmbedderKey 是方舟客户端复用缓存的键：配置一致即可共用连接池。
type arkEmbedderKey struct {
	apiKey  string
	model   string
	dims    int
	timeout time.Duration
}

// ArkEmbedderFactory 按配置复用方舟客户端，避免每次索引/检索都重建连接池。
//
// 原实现用包级 map + 包级 mutex 做缓存（违反「无包级可变单例」），
// 此处改为可注入类型，由组合根创建唯一实例。
type ArkEmbedderFactory struct {
	mu    sync.Mutex
	cache map[arkEmbedderKey]*ArkMultimodalEmbedder
}

// NewArkEmbedderFactory 构造方舟向量客户端工厂。
func NewArkEmbedderFactory() *ArkEmbedderFactory {
	return &ArkEmbedderFactory{cache: make(map[arkEmbedderKey]*ArkMultimodalEmbedder)}
}

// Embedder 返回与配置对应的多模态向量客户端（同配置复用同一实例）。
func (f *ArkEmbedderFactory) Embedder(cfg app.VectorConfig) *ArkMultimodalEmbedder {
	timeout := defaultEmbeddingTimeout
	if cfg.TimeoutSec > 0 {
		timeout = time.Duration(cfg.TimeoutSec) * time.Second
	}

	key := arkEmbedderKey{
		apiKey:  cfg.EmbeddingAPIKey,
		model:   cfg.EmbeddingModel,
		dims:    cfg.EmbeddingDimensions,
		timeout: timeout,
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if cached, ok := f.cache[key]; ok {
		return cached
	}

	embedder := &ArkMultimodalEmbedder{
		client: arkruntime.NewClientWithApiKey(
			cfg.EmbeddingAPIKey,
			arkruntime.WithTimeout(timeout),
			arkruntime.WithRetryTimes(embeddingRetryTimes),
		),
		model: cfg.EmbeddingModel,
		dims:  cfg.EmbeddingDimensions,
	}
	f.cache[key] = embedder

	return embedder
}
