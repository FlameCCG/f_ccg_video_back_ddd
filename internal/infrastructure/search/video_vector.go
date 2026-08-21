package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	app "f_ccg_video_back_ddd/internal/application/search"
	domain "f_ccg_video_back_ddd/internal/domain/search"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"

	elasticsearch8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"go.uber.org/zap"
)

const (
	defaultEmbeddingDims   = 2048
	defaultRRFRankConstant = 60
	knnCandidateFactor     = 20
	knnMinCandidates       = 100
	keywordRecallFactor    = 3
	vectorErrorBodyLimit   = 4096
	coverFetchTimeout      = 10 * time.Second
	vectorFieldVideoID     = "video_id"
	vectorFieldTitle       = "title"
	vectorFieldDesc        = "description"
	vectorFieldTags        = "tags"
	vectorFieldPartition   = "partition"
	vectorFieldCover       = "cover"
	vectorFieldStatus      = "status"
	vectorFieldContent     = "content"
	vectorFieldEmbedding   = "content_vector"
	vectorFieldCreatedAt   = "created_at"
	vectorFieldHasImage    = "has_image_vector"
	esMappingType          = "type"
	esMappingText          = "text"
	esMappingKeyword       = "keyword"
	embeddingImageModeOn   = "on"
	embeddingImageModeOff  = "off"
)

type vectorHit struct {
	VideoID uint
	Score   float64
}

type vectorIndexState struct {
	index string
	dims  int
}

// VideoVectorIndex 是视频多模态向量索引的 ES8 + 方舟适配器。
// 客户端、embedding 连接池与索引校验缓存均为实例状态，不依赖包级可变单例。
type VideoVectorIndex struct {
	client      *elasticsearch8.Client
	coverClient *http.Client
	logger      *zap.Logger

	embedMu   sync.Mutex
	embedders map[arkEmbedderKey]*arkEmbedder

	indexMu sync.Mutex
	checked map[vectorIndexState]struct{}
}

var _ app.VideoVectorIndex = (*VideoVectorIndex)(nil)
var _ app.VideoVectorIndexLifecycle = (*VideoVectorIndex)(nil)

// NewVideoVectorIndex 构造视频向量索引适配器（client 允许为 nil）。
func NewVideoVectorIndex(client *elasticsearch8.Client, logger *zap.Logger) *VideoVectorIndex {
	return &VideoVectorIndex{
		client:      client,
		coverClient: &http.Client{Timeout: coverFetchTimeout},
		logger:      logger,
		embedders:   make(map[arkEmbedderKey]*arkEmbedder),
		checked:     make(map[vectorIndexState]struct{}),
	}
}

// Available 报告 ES8 客户端是否就绪。
func (v *VideoVectorIndex) Available() bool {
	return v != nil && v.client != nil
}

// SearchIDs 对文本、图片或二者组合执行 kNN + BM25 召回，并在应用侧用 RRF 融合。
func (v *VideoVectorIndex) SearchIDs(
	ctx context.Context,
	cfg app.VectorConfig,
	input app.EmbedInput,
	limit int,
) ([]uint, error) {
	if !v.Available() {
		return nil, app.ErrES8Unavailable
	}

	input.Text = strings.TrimSpace(input.Text)
	input.ImageURL = strings.TrimSpace(input.ImageURL)
	if input.IsZero() {
		return nil, nil
	}

	vector, err := v.embedder(cfg).Embed(ctx, input)
	if err != nil {
		return nil, err
	}

	knnHits, err := v.runKnnSearch(ctx, cfg, vector, limit)
	if err != nil {
		return nil, err
	}

	if input.Text == "" {
		return topVectorHitIDs(knnHits, limit), nil
	}

	keywordHits, err := v.runKeywordSearch(ctx, cfg, input.Text, limit*keywordRecallFactor)
	if err != nil {
		v.logger.Warn(
			"keyword recall on vector index failed, falling back to vector-only",
			zap.String("index", cfg.Index),
			zap.Error(err),
		)

		return topVectorHitIDs(knnHits, limit), nil
	}

	return fuseByRRF(cfg.RRFRankConstant, limit, knnHits, keywordHits), nil
}

func (v *VideoVectorIndex) runKnnSearch(
	ctx context.Context,
	cfg app.VectorConfig,
	vector []float32,
	limit int,
) ([]vectorHit, error) {
	knn := map[string]any{
		"field":          vectorFieldEmbedding,
		"query_vector":   vector,
		"k":              limit,
		"num_candidates": max(limit*knnCandidateFactor, knnMinCandidates),
		"filter":         []any{publishedStatusFilter()},
	}
	if cfg.MinSimilarity > 0 {
		// Elasticsearch 的 cosine _score 为 (1 + cosine) / 2。
		knn["similarity"] = (1 + cfg.MinSimilarity) / 2
	}

	return v.runIndexSearch(ctx, cfg.Index, map[string]any{
		"size":    limit,
		"_source": []string{vectorFieldVideoID},
		"knn":     knn,
	})
}

func (v *VideoVectorIndex) runKeywordSearch(
	ctx context.Context,
	cfg app.VectorConfig,
	query string,
	limit int,
) ([]vectorHit, error) {
	return v.runIndexSearch(ctx, cfg.Index, map[string]any{
		"size":    limit,
		"_source": []string{vectorFieldVideoID},
		"query": map[string]any{
			"bool": map[string]any{
				"filter": []any{publishedStatusFilter()},
				"must": []any{map[string]any{
					"multi_match": map[string]any{
						"query": query,
						"fields": []string{
							vectorFieldTitle + "^3",
							vectorFieldTags + "^2",
							vectorFieldPartition + "^2",
							vectorFieldDesc,
							vectorFieldContent,
						},
					},
				}},
			},
		},
	})
}

func publishedStatusFilter() map[string]any {
	return map[string]any{
		"term": map[string]any{
			vectorFieldStatus: map[string]any{"value": int(enum.VideoStatusPublished)},
		},
	}
}

func (v *VideoVectorIndex) runIndexSearch(
	ctx context.Context,
	index string,
	body map[string]any,
) ([]vectorHit, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal vector search body failed: %w", err)
	}

	resp, err := (esapi.SearchRequest{
		Index: []string{index},
		Body:  bytes.NewReader(payload),
	}).Do(ctx, v.client)
	if err != nil {
		return nil, fmt.Errorf("vector search request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.IsError() {
		return nil, fmt.Errorf("vector search failed: %s %s", resp.Status(), readErrorBody(resp.Body))
	}

	var parsed struct {
		Hits struct {
			Hits []struct {
				Score  float64 `json:"_score"`
				Source struct {
					VideoID json.Number `json:"video_id"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode vector search response failed: %w", err)
	}

	hits := make([]vectorHit, 0, len(parsed.Hits.Hits))
	for _, hit := range parsed.Hits.Hits {
		id, convertErr := hit.Source.VideoID.Int64()
		if convertErr != nil || id <= 0 {
			continue
		}
		hits = append(hits, vectorHit{VideoID: uint(id), Score: hit.Score})
	}

	return hits, nil
}

// Store 覆盖写入一个已发布视频的文本+封面融合向量文档。
func (v *VideoVectorIndex) Store(
	ctx context.Context,
	cfg app.VectorConfig,
	source app.VideoVectorSource,
) error {
	if !v.Available() {
		return app.ErrES8Unavailable
	}
	if err := v.Prepare(ctx, cfg, false); err != nil {
		return err
	}

	document, content := buildVideoVectorDocument(source)
	imageSource, imageErr := v.resolveVideoCover(ctx, cfg, source.Cover)
	if imageErr != nil {
		if cfg.ImageMode == embeddingImageModeOn {
			return fmt.Errorf(
				"video %d cover embedding is required but unavailable: %w",
				source.ID,
				imageErr,
			)
		}
		v.logger.Debug("index video without cover vector", zap.Uint("videoId", source.ID), zap.Error(imageErr))
	}

	vector, err := v.embedder(cfg).Embed(ctx, app.EmbedInput{Text: content, ImageURL: imageSource})
	if err != nil {
		return fmt.Errorf("embed video %d failed: %w", source.ID, err)
	}
	document[vectorFieldEmbedding] = vector
	document[vectorFieldHasImage] = imageSource != ""

	payload, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("marshal video vector document failed: %w", err)
	}

	resp, err := (esapi.IndexRequest{
		Index:      cfg.Index,
		DocumentID: domain.VideoDocumentID(source.ID),
		Body:       bytes.NewReader(payload),
	}).Do(ctx, v.client)
	if err != nil {
		return fmt.Errorf("index video vector request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.IsError() {
		return fmt.Errorf(
			"index video vector document failed: %s %s",
			resp.Status(),
			readErrorBody(resp.Body),
		)
	}

	return nil
}

func (v *VideoVectorIndex) resolveVideoCover(
	ctx context.Context,
	cfg app.VectorConfig,
	cover string,
) (string, error) {
	if cfg.ImageMode == embeddingImageModeOff {
		return "", nil
	}

	return resolveCoverEmbedSource(ctx, v.coverClient, cover, cfg.AssetBaseURL, cfg.LocalStorageRoot)
}

// Delete 删除一个视频向量文档；索引或文档不存在均视为成功。
func (v *VideoVectorIndex) Delete(ctx context.Context, cfg app.VectorConfig, videoID uint) error {
	if !v.Available() {
		return app.ErrES8Unavailable
	}

	resp, err := (esapi.DeleteRequest{
		Index:      cfg.Index,
		DocumentID: domain.VideoDocumentID(videoID),
		Refresh:    "true",
	}).Do(ctx, v.client)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.IsError() {
		return fmt.Errorf(
			"delete video vector document failed: %s %s",
			resp.Status(),
			readErrorBody(resp.Body),
		)
	}

	return nil
}

// Prepare 确保索引存在，并校验已有 dense_vector mapping 的维度。
func (v *VideoVectorIndex) Prepare(ctx context.Context, cfg app.VectorConfig, recreate bool) error {
	if !v.Available() {
		return app.ErrES8Unavailable
	}

	if recreate {
		ignoreUnavailable := true
		resp, err := (esapi.IndicesDeleteRequest{
			Index:             []string{cfg.Index},
			IgnoreUnavailable: &ignoreUnavailable,
		}).Do(ctx, v.client)
		if err != nil {
			return fmt.Errorf("删除向量索引失败: %w", err)
		}
		if err = closeVectorResponse(resp, "删除向量索引失败"); err != nil {
			return err
		}
		v.resetIndexChecks()
	}

	state := vectorIndexState{index: cfg.Index, dims: cfg.EmbeddingDimensions}
	v.indexMu.Lock()
	_, checked := v.checked[state]
	v.indexMu.Unlock()
	if checked {
		return nil
	}

	exists, err := v.indexExists(ctx, cfg.Index)
	if err != nil {
		return err
	}
	if !exists {
		if err = v.createIndex(ctx, cfg); err != nil {
			return err
		}
	} else if err = v.verifyIndexDimensions(ctx, cfg); err != nil {
		return err
	}

	v.indexMu.Lock()
	v.checked[state] = struct{}{}
	v.indexMu.Unlock()

	return nil
}

func (v *VideoVectorIndex) indexExists(ctx context.Context, index string) (bool, error) {
	resp, err := (esapi.IndicesExistsRequest{Index: []string{index}}).Do(ctx, v.client)
	if err != nil {
		return false, fmt.Errorf("check vector index existence failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.IsError() {
		return false, fmt.Errorf("check vector index existence failed: %s", resp.Status())
	}

	return true, nil
}

func (v *VideoVectorIndex) createIndex(ctx context.Context, cfg app.VectorConfig) error {
	payload, err := json.Marshal(buildVideoVectorIndexSpec(cfg.EmbeddingDimensions))
	if err != nil {
		return fmt.Errorf("marshal vector index spec failed: %w", err)
	}

	resp, err := (esapi.IndicesCreateRequest{
		Index: cfg.Index,
		Body:  bytes.NewReader(payload),
	}).Do(ctx, v.client)
	if err != nil {
		return fmt.Errorf("create vector index failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.IsError() {
		body := readErrorBody(resp.Body)
		if strings.Contains(body, "resource_already_exists_exception") {
			return nil
		}

		return fmt.Errorf("create vector index failed: %s %s", resp.Status(), body)
	}

	v.logger.Info(
		"vector index created",
		zap.String("index", cfg.Index),
		zap.Int("dims", cfg.EmbeddingDimensions),
	)

	return nil
}

func (v *VideoVectorIndex) verifyIndexDimensions(ctx context.Context, cfg app.VectorConfig) error {
	resp, err := (esapi.IndicesGetMappingRequest{Index: []string{cfg.Index}}).Do(ctx, v.client)
	if err != nil {
		return fmt.Errorf("read vector index mapping failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.IsError() {
		return fmt.Errorf("read vector index mapping failed: %s %s", resp.Status(), readErrorBody(resp.Body))
	}

	var parsed map[string]struct {
		Mappings struct {
			Properties struct {
				ContentVector struct {
					Dims int `json:"dims"`
				} `json:"content_vector"`
			} `json:"properties"`
		} `json:"mappings"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fmt.Errorf("decode vector index mapping failed: %w", err)
	}

	for _, entry := range parsed {
		dims := entry.Mappings.Properties.ContentVector.Dims
		if dims > 0 && dims != cfg.EmbeddingDimensions {
			return fmt.Errorf(
				"vector index %q was built with dims=%d but embeddingDimensions=%d; "+
					"ES 不支持原地修改 dense_vector 维度，请删除索引后调用重建接口",
				cfg.Index,
				dims,
				cfg.EmbeddingDimensions,
			)
		}
	}

	return nil
}

func (v *VideoVectorIndex) resetIndexChecks() {
	v.indexMu.Lock()
	v.checked = make(map[vectorIndexState]struct{})
	v.indexMu.Unlock()
}

// Refresh 让本次写入立即可检索。
func (v *VideoVectorIndex) Refresh(ctx context.Context, cfg app.VectorConfig) error {
	if !v.Available() {
		return app.ErrES8Unavailable
	}

	resp, err := (esapi.IndicesRefreshRequest{Index: []string{cfg.Index}}).Do(ctx, v.client)
	if err != nil {
		return fmt.Errorf("刷新向量索引失败: %w", err)
	}

	return closeVectorResponse(resp, "刷新向量索引失败")
}

func closeVectorResponse(response *esapi.Response, operation string) error {
	if response == nil {
		return fmt.Errorf("%s: empty response", operation)
	}
	defer func() { _ = response.Body.Close() }()

	if !response.IsError() {
		return nil
	}

	return fmt.Errorf("%s: %s %s", operation, response.Status(), readErrorBody(response.Body))
}

func buildVideoVectorIndexSpec(dimensions int) map[string]any {
	if dimensions <= 0 {
		dimensions = defaultEmbeddingDims
	}

	return map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]any{
			"properties": map[string]any{
				vectorFieldVideoID: map[string]any{esMappingType: "long"},
				vectorFieldTitle: map[string]any{
					esMappingType: esMappingText,
					"fields": map[string]any{
						esMappingKeyword: map[string]any{esMappingType: esMappingKeyword},
					},
				},
				vectorFieldDesc: map[string]any{esMappingType: esMappingText},
				vectorFieldTags: map[string]any{
					esMappingType: esMappingText,
					"fields": map[string]any{
						esMappingKeyword: map[string]any{esMappingType: esMappingKeyword},
					},
				},
				vectorFieldPartition: map[string]any{
					esMappingType: esMappingText,
					"fields": map[string]any{
						esMappingKeyword: map[string]any{esMappingType: esMappingKeyword},
					},
				},
				vectorFieldCover:     map[string]any{esMappingType: esMappingKeyword, "index": false},
				vectorFieldStatus:    map[string]any{esMappingType: "integer"},
				vectorFieldContent:   map[string]any{esMappingType: esMappingText},
				vectorFieldCreatedAt: map[string]any{esMappingType: "date"},
				vectorFieldHasImage:  map[string]any{esMappingType: "boolean"},
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

func buildVideoVectorDocument(source app.VideoVectorSource) (map[string]any, string) {
	tagNames := domain.TrimNonEmpty(source.Tags)
	content := (domain.VideoVectorInput{
		Title:       source.Title,
		Description: source.Description,
		Partition:   source.Partition,
		Tags:        source.Tags,
		PartTitles:  source.PartTitles,
	}).BuildContent()

	return map[string]any{
		vectorFieldVideoID:   source.ID,
		vectorFieldTitle:     strings.TrimSpace(source.Title),
		vectorFieldDesc:      strings.TrimSpace(source.Description),
		vectorFieldTags:      tagNames,
		vectorFieldPartition: strings.TrimSpace(source.Partition),
		vectorFieldCover:     strings.TrimSpace(source.Cover),
		vectorFieldStatus:    int(source.Status),
		vectorFieldCreatedAt: source.CreatedAt.Format(time.RFC3339),
		vectorFieldContent:   content,
	}, content
}

func fuseByRRF(rankConstant, limit int, rankings ...[]vectorHit) []uint {
	if rankConstant <= 0 {
		rankConstant = defaultRRFRankConstant
	}

	scores := make(map[uint]float64)
	order := make([]uint, 0, limit*len(rankings))
	for _, ranking := range rankings {
		for rank, hit := range ranking {
			if _, seen := scores[hit.VideoID]; !seen {
				order = append(order, hit.VideoID)
			}
			scores[hit.VideoID] += 1 / float64(rankConstant+rank+1)
		}
	}

	position := make(map[uint]int, len(order))
	for index, id := range order {
		position[id] = index
	}
	sort.SliceStable(order, func(left, right int) bool {
		leftID, rightID := order[left], order[right]
		if scores[leftID] != scores[rightID] {
			return scores[leftID] > scores[rightID]
		}

		return position[leftID] < position[rightID]
	})

	if limit > 0 && len(order) > limit {
		order = order[:limit]
	}

	return order
}

func topVectorHitIDs(hits []vectorHit, limit int) []uint {
	ids := make([]uint, 0, len(hits))
	seen := make(map[uint]struct{}, len(hits))
	for _, hit := range hits {
		if _, ok := seen[hit.VideoID]; ok {
			continue
		}
		seen[hit.VideoID] = struct{}{}
		ids = append(ids, hit.VideoID)
		if limit > 0 && len(ids) >= limit {
			break
		}
	}

	return ids
}

func readErrorBody(body io.Reader) string {
	raw, err := io.ReadAll(io.LimitReader(body, vectorErrorBodyLimit))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(raw))
}
