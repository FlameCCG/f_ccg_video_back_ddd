package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	app "f_ccg_video_back_ddd/internal/application/search"
	domain "f_ccg_video_back_ddd/internal/domain/search"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"

	arkembedding "github.com/cloudwego/eino-ext/components/embedding/ark"
	indexeres8 "github.com/cloudwego/eino-ext/components/indexer/es8"
	retrieveres8 "github.com/cloudwego/eino-ext/components/retriever/es8"
	"github.com/cloudwego/eino-ext/components/retriever/es8/search_mode"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	elasticsearch8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	estypes "github.com/elastic/go-elasticsearch/v8/typedapi/types"
)

// 向量索引字段名（与 buildVideoVectorIndexSpec 的 mapping 一一对应，属于索引契约）.
const (
	vectorFieldVideoID   = "video_id"
	vectorFieldTitle     = "title"
	vectorFieldDesc      = "description"
	vectorFieldTags      = "tags"
	vectorFieldPartition = "partition"
	vectorFieldCover     = "cover"
	vectorFieldStatus    = "status"
	vectorFieldContent   = "content"
	vectorFieldEmbedding = "content_vector"
	vectorFieldCreatedAt = "created_at"

	esMappingType    = "type"
	esMappingText    = "text"
	esMappingKeyword = "keyword"
)

const (
	// defaultEmbeddingDims 未配置维度时的向量维度（与应用层默认值一致）.
	defaultEmbeddingDims = 2048
	// defaultEmbeddingTimeout 未配置 timeoutSec 时的向量模型调用超时.
	defaultEmbeddingTimeout = 30 * time.Second
	// vectorIndexBatchSize eino 索引器的批量大小.
	vectorIndexBatchSize = 4
	// vectorNumCandidatesFactor 近似检索候选放大倍数.
	vectorNumCandidatesFactor = 20
	// vectorNumCandidatesMin 近似检索候选下限.
	vectorNumCandidatesMin = 100
	// vectorErrorBodyLimit 读取 ES 错误响应体的上限.
	vectorErrorBodyLimit = 4096
)

// VideoVectorIndex 是视频向量索引的 ES8 + eino-ext 适配器（原 service/search_service/video_vector.go）。
//
// 索引名与维度来自运行期 AI 配置（由应用层读取后传入），mapping 与字段名属于索引契约。
type VideoVectorIndex struct {
	client *elasticsearch8.Client
}

var _ app.VideoVectorIndex = (*VideoVectorIndex)(nil)

// NewVideoVectorIndex 构造视频向量索引适配器（client 允许为 nil）。
func NewVideoVectorIndex(client *elasticsearch8.Client) *VideoVectorIndex {
	return &VideoVectorIndex{client: client}
}

// Available 报告 ES8 客户端是否就绪。
func (v *VideoVectorIndex) Available() bool {
	return v != nil && v.client != nil
}

// SearchIDs 以文本 query 做近似向量检索（混合检索），返回去重后的视频 ID。
func (v *VideoVectorIndex) SearchIDs(
	ctx context.Context,
	cfg app.VectorConfig,
	query string,
	limit int,
) ([]uint, error) {
	if !v.Available() {
		return nil, app.ErrES8Unavailable
	}

	embedder, err := newArkMultimodalEmbedder(ctx, cfg)
	if err != nil {
		return nil, err
	}

	k := limit

	numCandidates := max(limit*vectorNumCandidatesFactor, vectorNumCandidatesMin)

	retriever, err := retrieveres8.NewRetriever(ctx, &retrieveres8.RetrieverConfig{
		Client: v.client,
		Index:  cfg.Index,
		TopK:   limit,
		SearchMode: search_mode.SearchModeApproximate(&search_mode.ApproximateConfig{
			QueryFieldName:  vectorFieldContent,
			VectorFieldName: vectorFieldEmbedding,
			Hybrid:          true,
			RRF:             false,
			K:               &k,
			NumCandidates:   &numCandidates,
		}),
		ResultParser: parseVectorSearchHit,
		Embedding:    embedder,
	})
	if err != nil {
		return nil, err
	}

	docs, err := retriever.Retrieve(ctx, query, retrieveres8.WithFilters([]estypes.Query{
		{
			Term: map[string]estypes.TermQuery{
				vectorFieldStatus: {Value: int(enum.VideoStatusPublished)},
			},
		},
	}))
	if err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(docs))

	seen := make(map[uint]struct{}, len(docs))

	for _, doc := range docs {
		if doc == nil || doc.MetaData == nil {
			continue
		}

		id := toUint(doc.MetaData[vectorFieldVideoID])
		if id == 0 {
			continue
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}

		ids = append(ids, id)

		if len(ids) >= limit {
			break
		}
	}

	return ids, nil
}

// Store 覆盖写入一个视频的向量文档。
func (v *VideoVectorIndex) Store(ctx context.Context, cfg app.VectorConfig, source app.VideoVectorSource) error {
	if !v.Available() {
		return app.ErrES8Unavailable
	}

	indexer, err := newVideoVectorIndexer(ctx, v.client, cfg)
	if err != nil {
		return err
	}

	_, err = indexer.Store(ctx, []*schema.Document{buildVideoVectorDocument(source)})

	return err
}

// Delete 删除一个视频的向量文档；文档不存在视为成功。
func (v *VideoVectorIndex) Delete(ctx context.Context, cfg app.VectorConfig, videoID uint) error {
	if !v.Available() {
		return app.ErrES8Unavailable
	}

	req := esapi.DeleteRequest{
		Index:      cfg.Index,
		DocumentID: domain.VideoDocumentID(videoID),
		Refresh:    "true",
	}

	resp, err := req.Do(ctx, v.client)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.IsError() {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, vectorErrorBodyLimit))

		return fmt.Errorf("delete video vector document failed: %s %s", resp.Status(), strings.TrimSpace(string(body)))
	}

	return nil
}

func newVideoVectorIndexer(
	ctx context.Context,
	client *elasticsearch8.Client,
	cfg app.VectorConfig,
) (*indexeres8.Indexer, error) {
	embedder, err := newArkMultimodalEmbedder(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return indexeres8.NewIndexer(ctx, &indexeres8.IndexerConfig{
		Client:    client,
		Index:     cfg.Index,
		IndexSpec: buildVideoVectorIndexSpec(cfg.EmbeddingDimensions),
		BatchSize: vectorIndexBatchSize,
		DocumentToFields: func(_ context.Context, doc *schema.Document) (map[string]indexeres8.FieldValue, error) {
			return documentToVideoVectorFields(doc), nil
		},
		Embedding: embedder,
	})
}

func buildVideoVectorIndexSpec(dimensions int) *indexeres8.IndexSpec {
	if dimensions <= 0 {
		dimensions = defaultEmbeddingDims
	}

	return &indexeres8.IndexSpec{
		Settings: map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		Mappings: map[string]any{
			"properties": map[string]any{
				vectorFieldVideoID: map[string]any{esMappingType: "long"},
				vectorFieldTitle: map[string]any{
					esMappingType: esMappingText,
					"fields":      map[string]any{esMappingKeyword: map[string]any{esMappingType: esMappingKeyword}},
				},
				vectorFieldDesc:      map[string]any{esMappingType: esMappingText},
				vectorFieldTags:      map[string]any{esMappingType: esMappingKeyword},
				vectorFieldPartition: map[string]any{esMappingType: esMappingKeyword},
				vectorFieldCover:     map[string]any{esMappingType: esMappingKeyword, "index": false},
				vectorFieldStatus:    map[string]any{esMappingType: "integer"},
				vectorFieldContent:   map[string]any{esMappingType: esMappingText},
				vectorFieldCreatedAt: map[string]any{esMappingType: "date"},
				vectorFieldEmbedding: map[string]any{
					esMappingType: "dense_vector",
					"dims":        dimensions,
					"index":       true,
					"similarity":  "cosine",
				},
			},
		},
	}
}

func documentToVideoVectorFields(doc *schema.Document) map[string]indexeres8.FieldValue {
	meta := doc.MetaData

	return map[string]indexeres8.FieldValue{
		vectorFieldVideoID:   {Value: meta[vectorFieldVideoID]},
		vectorFieldTitle:     {Value: meta[vectorFieldTitle]},
		vectorFieldDesc:      {Value: meta[vectorFieldDesc]},
		vectorFieldTags:      {Value: meta[vectorFieldTags]},
		vectorFieldPartition: {Value: meta[vectorFieldPartition]},
		vectorFieldCover:     {Value: meta[vectorFieldCover]},
		vectorFieldStatus:    {Value: meta[vectorFieldStatus]},
		vectorFieldCreatedAt: {Value: meta[vectorFieldCreatedAt]},
		vectorFieldContent: {
			Value:    doc.Content,
			EmbedKey: vectorFieldEmbedding,
			// 官方 ark.Embedder 仅支持文本 EmbedStrings；封面图仅作元数据存储。
			Stringify: func(value any) (string, error) {
				return fmt.Sprint(value), nil
			},
		},
	}
}

func parseVectorSearchHit(_ context.Context, hit estypes.Hit) (*schema.Document, error) {
	if hit.Id_ == nil {
		return nil, errors.New("vector search hit missing _id")
	}

	if hit.Source_ == nil {
		return nil, errors.New("vector search hit missing _source")
	}

	var source map[string]any
	if err := json.Unmarshal(hit.Source_, &source); err != nil {
		return nil, err
	}

	content := toString(source[vectorFieldContent])

	meta := make(map[string]any, len(source)+1)

	for key, value := range source {
		if key == vectorFieldEmbedding || key == vectorFieldContent {
			continue
		}

		meta[key] = value
	}

	if hit.Score_ != nil {
		meta["score"] = float64(*hit.Score_)
	}

	return (&schema.Document{
		ID:       *hit.Id_,
		Content:  content,
		MetaData: meta,
	}).WithScore(toFloat64(meta["score"])), nil
}

// buildVideoVectorDocument 把库内视频数据组装成向量文档。
func buildVideoVectorDocument(source app.VideoVectorSource) *schema.Document {
	tagNames := domain.TrimNonEmpty(source.Tags)
	input := domain.VideoVectorInput{
		Title:       source.Title,
		Description: source.Description,
		Partition:   source.Partition,
		Tags:        source.Tags,
		PartTitles:  source.PartTitles,
	}

	return &schema.Document{
		ID:      domain.VideoDocumentID(source.ID),
		Content: input.BuildContent(),
		MetaData: map[string]any{
			vectorFieldVideoID:   source.ID,
			vectorFieldTitle:     strings.TrimSpace(source.Title),
			vectorFieldDesc:      strings.TrimSpace(source.Description),
			vectorFieldTags:      tagNames,
			vectorFieldPartition: strings.TrimSpace(source.Partition),
			vectorFieldCover:     strings.TrimSpace(source.Cover),
			vectorFieldStatus:    int(source.Status),
			vectorFieldCreatedAt: source.CreatedAt.Format(time.RFC3339),
		},
	}
}

// newArkMultimodalEmbedder 使用 Eino 官方 embedding/ark（默认 BaseURL，多模态文本 API）。
func newArkMultimodalEmbedder(ctx context.Context, cfg app.VectorConfig) (embedding.Embedder, error) {
	apiType := arkembedding.APITypeMultiModal
	dims := cfg.EmbeddingDimensions
	timeout := defaultEmbeddingTimeout

	if cfg.TimeoutSec > 0 {
		timeout = time.Duration(cfg.TimeoutSec) * time.Second
	}

	return arkembedding.NewEmbedder(ctx, &arkembedding.EmbeddingConfig{
		APIKey:     cfg.EmbeddingAPIKey,
		Model:      cfg.EmbeddingModel,
		APIType:    &apiType,
		Dimensions: &dims,
		Timeout:    &timeout,
	})
}
