package search

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/apperr"
	"fmt"
	"strings"
	"time"

	app "f_ccg_video_back_ddd/internal/application/search"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	arkmodel "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

const (
	testAlice      = "alice"
	responseFields = "fields"
	fieldUsername  = "username"
)

const (
	embeddingRetryTimes    = 1
	defaultEmbedderTimeout = 20 * time.Second
)

type arkEmbedder struct {
	client *arkruntime.Client
	model  string
	dims   int
}

type arkEmbedderKey struct {
	apiKey  string
	model   string
	dims    int
	timeout time.Duration
}

func (v *VideoVectorIndex) embedder(cfg app.VectorConfig) *arkEmbedder {
	timeout := defaultEmbedderTimeout
	if cfg.TimeoutSec > 0 {
		timeout = time.Duration(cfg.TimeoutSec) * time.Second
	}

	key := arkEmbedderKey{
		apiKey:  cfg.EmbeddingAPIKey,
		model:   cfg.EmbeddingModel,
		dims:    cfg.EmbeddingDimensions,
		timeout: timeout,
	}

	v.embedMu.Lock()
	defer v.embedMu.Unlock()

	if cached, ok := v.embedders[key]; ok {
		return cached
	}

	embedder := &arkEmbedder{
		client: arkruntime.NewClientWithApiKey(
			cfg.EmbeddingAPIKey,
			arkruntime.WithTimeout(timeout),
			arkruntime.WithRetryTimes(embeddingRetryTimes),
		),
		model: cfg.EmbeddingModel,
		dims:  cfg.EmbeddingDimensions,
	}
	v.embedders[key] = embedder

	return embedder
}

// Embed 把文本与图片融合成一个稠密向量。
func (e *arkEmbedder) Embed(ctx context.Context, input app.EmbedInput) ([]float32, error) {
	input.Text = strings.TrimSpace(input.Text)

	input.ImageURL = strings.TrimSpace(input.ImageURL)
	if input.IsZero() {
		return nil, errors.New("embedding input is empty")
	}

	inputs := make([]arkmodel.MultimodalEmbeddingInput, 0, 2)

	if input.Text != "" {
		text := input.Text
		inputs = append(inputs, arkmodel.MultimodalEmbeddingInput{
			Type: arkmodel.MultiModalEmbeddingInputTypeText,
			Text: &text,
		})
	}

	if input.ImageURL != "" {
		imageURL := input.ImageURL
		inputs = append(inputs, arkmodel.MultimodalEmbeddingInput{
			Type:     arkmodel.MultiModalEmbeddingInputTypeImageURL,
			ImageURL: &arkmodel.MultimodalEmbeddingImageURL{URL: imageURL},
		})
	}

	encodingFormat := arkmodel.EmbeddingEncodingFormatFloat

	request := arkmodel.MultiModalEmbeddingRequest{
		Input:          inputs,
		Model:          e.model,
		EncodingFormat: &encodingFormat,
	}
	if e.dims > 0 {
		dimensions := e.dims
		request.Dimensions = &dimensions
	}

	response, err := e.client.CreateMultiModalEmbeddings(ctx, request)
	if err != nil {
		return nil, apperr.Private("ai.embedding", fmt.Errorf("call ark multimodal embeddings failed: %w", err))
	}

	vector := response.Data.Embedding
	if len(vector) == 0 {
		return nil, errors.New("ark multimodal embeddings returned an empty vector")
	}

	if e.dims > 0 && len(vector) != e.dims {
		return nil, fmt.Errorf(
			"embedding dimension mismatch: configured=%d, got=%d; 请同步修改 embeddingDimensions 或重建向量索引",
			e.dims,
			len(vector),
		)
	}

	return vector, nil
}
