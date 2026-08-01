package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/config"

	"go.uber.org/zap"
)

const (
	// defaultVideoVectorIndex 未配置 vectorIndex 时使用的向量索引名.
	defaultVideoVectorIndex = "search_videos_vector"
	// defaultVideoVectorTopK 未配置 vectorTopK 时的召回上限.
	defaultVideoVectorTopK = 8
	// defaultVideoEmbeddingDims 未配置 embeddingDimensions 时的向量维度.
	defaultVideoEmbeddingDims = 2048
	// defaultVectorRRFRankConstant 是混合召回的默认 RRF 排名常数。
	defaultVectorRRFRankConstant = 60

	embeddingImageModeAuto = "auto"
	embeddingImageModeOn   = "on"
	embeddingImageModeOff  = "off"

	// vectorSearchTimeout 向量检索超时.
	vectorSearchTimeout = 20 * time.Second
	// vectorIndexTimeout 向量索引写入超时.
	vectorIndexTimeout = 60 * time.Second
	// vectorDeleteTimeout 向量文档删除超时.
	vectorDeleteTimeout = 5 * time.Second
)

// 向量配置缺失时的错误（文案与原 service/search_service/video_vector.go 一致）.
var (
	// ErrAIConfigMissing 表示 ai 配置段缺失.
	ErrAIConfigMissing = errors.New("AI config not initialized")
	// ErrEmbeddingModelMissing 表示未配置向量模型.
	ErrEmbeddingModelMissing = errors.New("embedding model not configured")
	// ErrEmbeddingAPIKeyMissing 表示未配置方舟 API Key.
	ErrEmbeddingAPIKeyMissing = errors.New("doubao embedding API key not configured")
	// ErrES8Unavailable 表示 ES8 客户端未初始化.
	ErrES8Unavailable = errors.New("ES8 client not initialized")
)

// VideoVectorService 视频向量索引用例服务（原 service/search_service/video_vector.go）。
//
// 写侧由 video 上下文在发布/下架时触发；读侧供 AI「找视频」做语义召回。
type VideoVectorService struct {
	cfg    *config.Provider
	index  VideoVectorIndex
	source VideoVectorSourceQuery
	logger *zap.Logger
}

// NewVideoVectorService 构造视频向量索引服务。
func NewVideoVectorService(
	cfgProvider *config.Provider,
	index VideoVectorIndex,
	source VideoVectorSourceQuery,
	logger *zap.Logger,
) *VideoVectorService {
	return &VideoVectorService{cfg: cfgProvider, index: index, source: source, logger: logger}
}

// SearchVideoVectorIDs 以文本 query 做向量召回，返回去重后的已发布视频 ID。
func (s *VideoVectorService) SearchVideoVectorIDs(ctx context.Context, query string, limit int) ([]uint, error) {
	return s.SearchVideoVectorIDsMultiModal(ctx, EmbedInput{Text: query}, limit)
}

// SearchVideoVectorIDsMultiModal 以文本、图片或二者组合做混合召回。
func (s *VideoVectorService) SearchVideoVectorIDsMultiModal(
	ctx context.Context,
	input EmbedInput,
	limit int,
) ([]uint, error) {
	input.Text = strings.TrimSpace(input.Text)
	input.ImageURL = strings.TrimSpace(input.ImageURL)
	if input.IsZero() {
		return nil, nil
	}

	if !s.index.Available() {
		return nil, ErrES8Unavailable
	}

	cfg, err := s.vectorConfig()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = cfg.TopK
	}

	if cfg.TopK > 0 && limit > cfg.TopK {
		limit = cfg.TopK
	}

	searchCtx, cancel := context.WithTimeout(ensureContext(ctx), vectorSearchTimeout)
	defer cancel()

	return s.index.SearchIDs(searchCtx, cfg, input, limit)
}

// IndexVideoVector 将已发布视频写入向量检索索引；非已发布或已删除时改为删除文档。
func (s *VideoVectorService) IndexVideoVector(ctx context.Context, videoID uint) error {
	if videoID == 0 || !s.source.Available() || !s.index.Available() {
		return nil
	}

	cfg, err := s.vectorConfig()
	if err != nil {
		return err
	}

	indexCtx, cancel := context.WithTimeout(ensureContext(ctx), vectorIndexTimeout)
	defer cancel()

	return s.indexVideoVectorWithConfig(indexCtx, cfg, videoID)
}

// indexVideoVectorWithConfig 用固定配置快照同步单个视频；全量重建据此避免热更新造成维度撕裂。
func (s *VideoVectorService) indexVideoVectorWithConfig(
	ctx context.Context,
	cfg VectorConfig,
	videoID uint,
) error {

	source, err := s.source.Load(ctx, videoID)
	if err != nil {
		if errors.Is(err, ErrVectorSourceNotFound) {
			return s.index.Delete(ctx, cfg, videoID)
		}

		return err
	}

	return s.indexVideoVectorSourceWithConfig(ctx, cfg, source)
}

// indexVideoVectorSourceWithConfig 用已预载的源数据同步单个视频。
// 全量重建按页批量预载关联数据后调用它，避免逐条回查数据库形成 N+1 查询。
func (s *VideoVectorService) indexVideoVectorSourceWithConfig(
	ctx context.Context,
	cfg VectorConfig,
	source VideoVectorSource,
) error {
	if source.Status != enum.VideoStatusPublished {
		return s.index.Delete(ctx, cfg, source.ID)
	}

	return s.index.Store(ctx, cfg, source)
}

// DeleteVideoVector 从向量检索索引删除视频文档（文档不存在视为成功）。
func (s *VideoVectorService) DeleteVideoVector(ctx context.Context, videoID uint) error {
	if videoID == 0 || !s.index.Available() {
		return nil
	}

	cfg, err := s.vectorConfig()
	if err != nil {
		return err
	}

	deleteCtx, cancel := context.WithTimeout(ensureContext(ctx), vectorDeleteTimeout)
	defer cancel()

	return s.index.Delete(deleteCtx, cfg, videoID)
}

// IndexVideoVectorBestEffort 尽力同步向量索引，失败仅记日志.
func (s *VideoVectorService) IndexVideoVectorBestEffort(ctx context.Context, videoID uint) {
	if err := s.IndexVideoVector(ctx, videoID); err != nil {
		s.logger.Warn("sync video vector index failed", zap.Uint("videoId", videoID), zap.Error(err))
	}
}

// DeleteVideoVectorBestEffort 尽力删除向量索引文档，失败仅记日志.
func (s *VideoVectorService) DeleteVideoVectorBestEffort(ctx context.Context, videoID uint) {
	if err := s.DeleteVideoVector(ctx, videoID); err != nil {
		s.logger.Warn("delete video vector index failed", zap.Uint("videoId", videoID), zap.Error(err))
	}
}

// vectorConfig 读取运行期 AI 配置并补默认值；必填项缺失时报错。
func (s *VideoVectorService) vectorConfig() (VectorConfig, error) {
	if s.cfg == nil || s.cfg.Get() == nil || s.cfg.Get().AI == nil {
		return VectorConfig{}, ErrAIConfigMissing
	}

	snapshot := s.cfg.Get()
	source := *snapshot.AI

	index := strings.TrimSpace(source.VectorIndex)
	if index == "" {
		index = defaultVideoVectorIndex
	}

	topK := source.VectorTopK
	if topK <= 0 {
		topK = defaultVideoVectorTopK
	}

	dimensions := source.EmbeddingDimensions
	if dimensions <= 0 {
		dimensions = defaultVideoEmbeddingDims
	}
	if dimensions != 1024 && dimensions != defaultVideoEmbeddingDims {
		return VectorConfig{}, fmt.Errorf(
			"embeddingDimensions 只支持 1024 或 2048，当前为 %d", dimensions,
		)
	}

	model := strings.TrimSpace(source.EmbeddingModel)
	if model == "" {
		return VectorConfig{}, ErrEmbeddingModelMissing
	}

	apiKey := strings.TrimSpace(source.DoubaoModelAPIKey)
	if apiKey == "" {
		return VectorConfig{}, ErrEmbeddingAPIKeyMissing
	}

	rankConstant := source.VectorRRFRankConstant
	if rankConstant <= 0 {
		rankConstant = defaultVectorRRFRankConstant
	}

	localStorageRoot := ""
	if snapshot.Site != nil && snapshot.Site.Storage != nil && snapshot.Site.Storage.Local != nil &&
		snapshot.Site.Storage.Local.Enable {
		localStorageRoot = strings.TrimSpace(snapshot.Site.Storage.Local.Path)
	}

	return VectorConfig{
		Index:               index,
		TopK:                topK,
		EmbeddingModel:      model,
		EmbeddingAPIKey:     apiKey,
		EmbeddingDimensions: dimensions,
		TimeoutSec:          source.TimeoutSec,
		ImageMode:           normalizeEmbeddingImageMode(source.EmbeddingImageMode),
		AssetBaseURL:        strings.TrimSpace(source.AssetBaseURL),
		LocalStorageRoot:    localStorageRoot,
		RRFRankConstant:     rankConstant,
		MinSimilarity:       source.VectorMinSimilarity,
	}, nil
}

func normalizeEmbeddingImageMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case embeddingImageModeOn:
		return embeddingImageModeOn
	case embeddingImageModeOff:
		return embeddingImageModeOff
	default:
		return embeddingImageModeAuto
	}
}
