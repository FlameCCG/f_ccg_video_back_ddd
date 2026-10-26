package search

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"

	"go.uber.org/zap"
)

// 对应原 service/search_service/video_vector.go（原文件无单测，此处按分支补齐）。

// newVectorConfigProvider 造一份可用的 AI 配置（向量检索的必填项齐备）。
func newVectorConfigProvider(ai *config.AI) *infraconfig.Provider {
	return infraconfig.NewProvider(&config.Config{AI: ai})
}

func validVectorAIConfig() *config.AI {
	return &config.AI{
		EmbeddingModel:    "doubao-embedding",
		DoubaoModelAPIKey: "ak-test",
	}
}

// TestVectorConfigAppliesDefaults 索引名/TopK/维度缺省时补默认值，
// 默认值与原实现一致（search_videos_vector / 8 / 2048）。
func TestVectorConfigAppliesDefaults(t *testing.T) {
	t.Parallel()

	svc := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		&fakeVectorIndex{available: true},
		&fakeVectorSource{available: true}, zap.NewNop(),
	)

	cfg, err := svc.vectorConfig()
	require.NoError(t, err, "vectorConfig: %v", err)

	require.Equal(t, defaultVideoVectorIndex, cfg.Index, "index = %q, want %q", cfg.Index, defaultVideoVectorIndex)

	require.Equal(t, defaultVideoVectorTopK, cfg.TopK, "topK = %d, want %d", cfg.TopK, defaultVideoVectorTopK)

	require.Equal(t, defaultVideoEmbeddingDims, cfg.EmbeddingDimensions, "dims = %d, want %d", cfg.EmbeddingDimensions, defaultVideoEmbeddingDims)

	require.Equal(t, embeddingImageModeAuto, cfg.ImageMode, "defaults = image:%q rrf:%d", cfg.ImageMode, cfg.RRFRankConstant)
	require.Equal(t, defaultVectorRRFRankConstant, cfg.RRFRankConstant, "defaults = image:%q rrf:%d", cfg.ImageMode, cfg.RRFRankConstant)
}

// TestVectorConfigKeepsConfiguredValues 显式配置优先于默认值。
func TestVectorConfigKeepsConfiguredValues(t *testing.T) {
	t.Parallel()

	ai := validVectorAIConfig()
	ai.VectorIndex = "  custom_vector  "
	ai.VectorTopK = 3
	ai.EmbeddingDimensions = 1024
	ai.TimeoutSec = 7
	ai.EmbeddingImageMode = " ON "
	ai.AssetBaseURL = " https://cdn.example.com/ "
	ai.VectorRRFRankConstant = 42
	ai.VectorMinSimilarity = 0.35

	provider := infraconfig.NewProvider(&config.Config{
		AI: ai,
		Site: &site.Site{Storage: &site.Storage{Local: &site.Local{
			Enable: true,
			Path:   " uploads ",
		}}},
	})
	svc := NewVideoVectorService(provider, &fakeVectorIndex{}, &fakeVectorSource{}, zap.NewNop())

	cfg, err := svc.vectorConfig()
	require.NoError(t, err, "vectorConfig: %v", err)

	want := VectorConfig{
		Index:               "custom_vector",
		TopK:                3,
		EmbeddingModel:      "doubao-embedding",
		EmbeddingAPIKey:     "ak-test",
		EmbeddingDimensions: 1024,
		TimeoutSec:          7,
		ImageMode:           embeddingImageModeOn,
		AssetBaseURL:        "https://cdn.example.com/",
		LocalStorageRoot:    "uploads",
		RRFRankConstant:     42,
		MinSimilarity:       0.35,
	}
	require.Equal(t, want, cfg, "cfg = %+v, want %+v", cfg, want)
}

func TestVectorConfigRejectsMissingSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr error
	}{
		{name: "nil config", cfg: nil, wantErr: ErrAIConfigMissing},
		{name: "nil ai section", cfg: &config.Config{}, wantErr: ErrAIConfigMissing},
		{
			name:    "missing embedding model",
			cfg:     &config.Config{AI: &config.AI{DoubaoModelAPIKey: "ak"}},
			wantErr: ErrEmbeddingModelMissing,
		},
		{
			name:    "missing api key",
			cfg:     &config.Config{AI: &config.AI{EmbeddingModel: "m"}},
			wantErr: ErrEmbeddingAPIKeyMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := NewVideoVectorService(infraconfig.NewProvider(tt.cfg), &fakeVectorIndex{}, &fakeVectorSource{}, zap.NewNop())
			{
				_, err := svc.vectorConfig()
				require.ErrorIs(t, err, tt.wantErr, "err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestSearchVideoVectorIDsSkipsBlankQuery 空 query 直接返回 nil，不打 ES。
func TestSearchVideoVectorIDsSkipsBlankQuery(t *testing.T) {
	t.Parallel()

	index := &fakeVectorIndex{available: true}
	svc := NewVideoVectorService(newVectorConfigProvider(validVectorAIConfig()), index, &fakeVectorSource{}, zap.NewNop())

	ids, err := svc.SearchVideoVectorIDs(context.Background(), "   ", 5)
	require.NoError(t, err, "SearchVideoVectorIDs: %v", err)

	require.Nil(t, ids, "ids = %v, want nil", ids)

	require.Empty(t, index.searchCalls, "blank query must not reach the index: %v", index.searchCalls)
}

func TestSearchVideoVectorIDsRequiresES8(t *testing.T) {
	t.Parallel()

	svc := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		&fakeVectorIndex{available: false},
		&fakeVectorSource{available: true}, zap.NewNop(),
	)

	{
		_, err := svc.SearchVideoVectorIDs(context.Background(), "教程", 5)
		require.ErrorIs(t, err, ErrES8Unavailable, "err = %v, want ErrES8Unavailable", err)
	}
}

// TestSearchVideoVectorIDsClampsLimitToTopK limit 缺省取配置 TopK，且不超过配置的 TopK。
func TestSearchVideoVectorIDsClampsLimitToTopK(t *testing.T) {
	t.Parallel()

	ai := validVectorAIConfig()
	ai.VectorTopK = 3

	index := &fakeVectorIndex{available: true, ids: []uint{1, 2}}
	svc := NewVideoVectorService(newVectorConfigProvider(ai), index, &fakeVectorSource{available: true}, zap.NewNop())

	ids, err := svc.SearchVideoVectorIDs(context.Background(), "  教程  ", 50)
	require.NoError(t, err, "SearchVideoVectorIDs: %v", err)

	require.True(t, reflect.DeepEqual(ids, []uint{1, 2}), "ids = %v", ids)

	require.Equal(t, 3, index.searchLimit, "limit = %d, want 3 (clamped to TopK)", index.searchLimit)

	// query 传给索引前已 TrimSpace。
	require.True(t, reflect.DeepEqual(index.searchCalls, []EmbedInput{{Text: "教程"}}), "queries = %v", index.searchCalls)

	// limit<=0 时直接使用配置 TopK。
	{
		_, err := svc.SearchVideoVectorIDs(context.Background(), "教程", 0)
		require.NoError(t, err, "SearchVideoVectorIDs: %v", err)
	}

	require.Equal(t, 3, index.searchLimit, "default limit = %d, want 3", index.searchLimit)
}

func TestSearchVideoVectorIDsUsesConfiguredTopKAboveDefault(t *testing.T) {
	t.Parallel()

	ai := validVectorAIConfig()
	ai.VectorTopK = 12
	index := &fakeVectorIndex{available: true}
	svc := NewVideoVectorService(
		newVectorConfigProvider(ai),
		index,
		&fakeVectorSource{available: true},
		zap.NewNop(),
	)

	{
		_, err := svc.SearchVideoVectorIDs(context.Background(), "教程", 0)
		require.NoError(t, err, "SearchVideoVectorIDs: %v", err)
	}

	require.Equal(t, 12, index.searchLimit, "limit = %d, want configured TopK 12", index.searchLimit)
}

func TestNormalizeEmbeddingImageMode(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"":     embeddingImageModeAuto,
		"AUTO": embeddingImageModeAuto,
		//nolint:gocritic // Whitespace deliberately tests input normalization.
		" on ":    embeddingImageModeOn,
		"off":     embeddingImageModeOff,
		"unknown": embeddingImageModeAuto,
	}
	for input, want := range tests {
		{
			got := normalizeEmbeddingImageMode(input)
			require.Equal(t, want, got, "normalizeEmbeddingImageMode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSearchVideoVectorIDsMultiModalForwardsImage(t *testing.T) {
	t.Parallel()

	index := &fakeVectorIndex{available: true, ids: []uint{7}}
	svc := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		index,
		&fakeVectorSource{available: true}, zap.NewNop(),
	)

	ids, err := svc.SearchVideoVectorIDsMultiModal(context.Background(), EmbedInput{
		Text:     "  红色跑车 ",
		ImageURL: " https://cdn.example.com/query.jpg ",
	}, 2)
	require.NoError(t, err, "SearchVideoVectorIDsMultiModal: %v", err)

	require.True(t, reflect.DeepEqual(ids, []uint{7}), "ids = %v", ids)

	want := []EmbedInput{{Text: "红色跑车", ImageURL: "https://cdn.example.com/query.jpg"}}
	require.True(t, reflect.DeepEqual(index.searchCalls, want), "inputs = %+v, want %+v", index.searchCalls, want)
}

// TestIndexVideoVectorSkipsWhenDependenciesMissing videoID 为 0、DB 或 ES8 缺失时
// 静默跳过（返回 nil，不报错），与原实现一致。
func TestIndexVideoVectorSkipsWhenDependenciesMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		videoID uint
		index   *fakeVectorIndex
		source  *fakeVectorSource
	}{
		{name: "zero id", videoID: 0, index: &fakeVectorIndex{available: true}, source: &fakeVectorSource{available: true}},
		{name: "db missing", videoID: 1, index: &fakeVectorIndex{available: true}, source: &fakeVectorSource{available: false}},
		{name: "es8 missing", videoID: 1, index: &fakeVectorIndex{available: false}, source: &fakeVectorSource{available: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := NewVideoVectorService(newVectorConfigProvider(validVectorAIConfig()), tt.index, tt.source, zap.NewNop())
			{
				err := svc.IndexVideoVector(context.Background(), tt.videoID)
				require.NoError(t, err, "IndexVideoVector: %v", err)
			}

			require.Empty(t, tt.index.stored, "index touched: stored=%v deleted=%v", tt.index.stored, tt.index.deleted)
			require.Empty(t, tt.index.deleted, "index touched: stored=%v deleted=%v", tt.index.stored, tt.index.deleted)
		})
	}
}

// TestIndexVideoVectorStoresPublishedVideo 已发布视频写入向量索引。
func TestIndexVideoVectorStoresPublishedVideo(t *testing.T) {
	t.Parallel()

	source := VideoVectorSource{ID: 9, Title: "教程", Status: enum.VideoStatusPublished}
	index := &fakeVectorIndex{available: true}
	svc := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		index,
		&fakeVectorSource{available: true, source: source}, zap.NewNop(),
	)

	{
		err := svc.IndexVideoVector(context.Background(), 9)
		require.NoError(t, err, "IndexVideoVector: %v", err)
	}

	require.True(t, reflect.DeepEqual(index.stored, []VideoVectorSource{source}), "stored = %+v", index.stored)

	require.Empty(t, index.deleted, "published video must not be deleted: %v", index.deleted)
}

// TestIndexVideoVectorDeletesWhenNotPublished 非「已发布」状态改为删除文档
// （避免下架/私密视频仍被向量召回）。
func TestIndexVideoVectorDeletesWhenNotPublished(t *testing.T) {
	t.Parallel()

	index := &fakeVectorIndex{available: true}
	svc := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		index,
		&fakeVectorSource{available: true, source: VideoVectorSource{ID: 9, Status: enum.VideoStatusPrivate}}, zap.NewNop(),
	)

	{
		err := svc.IndexVideoVector(context.Background(), 9)
		require.NoError(t, err, "IndexVideoVector: %v", err)
	}

	require.True(t, reflect.DeepEqual(index.deleted, []uint{9}), "deleted = %v, want [9]", index.deleted)

	require.Empty(t, index.stored, "unpublished video must not be stored: %+v", index.stored)
}

// TestIndexVideoVectorDeletesWhenSourceMissing 源视频已被物理删除时删除向量文档。
func TestIndexVideoVectorDeletesWhenSourceMissing(t *testing.T) {
	t.Parallel()

	index := &fakeVectorIndex{available: true}
	svc := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		index,
		&fakeVectorSource{available: true, err: ErrVectorSourceNotFound}, zap.NewNop(),
	)

	{
		err := svc.IndexVideoVector(context.Background(), 9)
		require.NoError(t, err, "IndexVideoVector: %v", err)
	}

	require.True(t, reflect.DeepEqual(index.deleted, []uint{9}), "deleted = %v, want [9]", index.deleted)
}

func TestIndexVideoVectorPropagatesSourceError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("db down")
	svc := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		&fakeVectorIndex{available: true},
		&fakeVectorSource{available: true, err: wantErr}, zap.NewNop(),
	)

	{
		err := svc.IndexVideoVector(context.Background(), 9)
		require.ErrorIs(t, err, wantErr, "err = %v, want %v", err, wantErr)
	}
}

// TestIndexVideoVectorPropagatesConfigError 配置不全时报错，且不触碰索引。
func TestIndexVideoVectorPropagatesConfigError(t *testing.T) {
	t.Parallel()

	index := &fakeVectorIndex{available: true}
	svc := NewVideoVectorService(
		infraconfig.NewProvider(&config.Config{}),
		index,
		&fakeVectorSource{available: true}, zap.NewNop(),
	)

	{
		err := svc.IndexVideoVector(context.Background(), 9)
		require.ErrorIs(t, err, ErrAIConfigMissing, "err = %v, want ErrAIConfigMissing", err)
	}

	require.Empty(t, index.stored, "index touched: stored=%v deleted=%v", index.stored, index.deleted)
	require.Empty(t, index.deleted, "index touched: stored=%v deleted=%v", index.stored, index.deleted)
}

// TestDeleteVideoVectorSkipsWhenUnavailable videoID 为 0 或 ES8 缺失时静默跳过。
func TestDeleteVideoVectorSkipsWhenUnavailable(t *testing.T) {
	t.Parallel()

	index := &fakeVectorIndex{available: true}
	svc := NewVideoVectorService(newVectorConfigProvider(validVectorAIConfig()), index, &fakeVectorSource{}, zap.NewNop())

	{
		err := svc.DeleteVideoVector(context.Background(), 0)
		require.NoError(t, err, "DeleteVideoVector: %v", err)
	}

	offline := &fakeVectorIndex{available: false}
	svc = NewVideoVectorService(newVectorConfigProvider(validVectorAIConfig()), offline, &fakeVectorSource{}, zap.NewNop())

	{
		err := svc.DeleteVideoVector(context.Background(), 9)
		require.NoError(t, err, "DeleteVideoVector: %v", err)
	}

	require.Empty(t, index.deleted, "delete must be skipped: %v %v", index.deleted, offline.deleted)
	require.Empty(t, offline.deleted, "delete must be skipped: %v %v", index.deleted, offline.deleted)
}

func TestDeleteVideoVectorForwardsToIndex(t *testing.T) {
	t.Parallel()

	index := &fakeVectorIndex{available: true}
	svc := NewVideoVectorService(newVectorConfigProvider(validVectorAIConfig()), index, &fakeVectorSource{}, zap.NewNop())

	{
		err := svc.DeleteVideoVector(context.Background(), 9)
		require.NoError(t, err, "DeleteVideoVector: %v", err)
	}

	require.True(t, reflect.DeepEqual(index.deleted, []uint{9}), "deleted = %v, want [9]", index.deleted)
}

// TestBestEffortVariantsSwallowErrors BestEffort 版本只记日志，不向调用方传播错误
// （视频发布/下架流程不能因向量同步失败而中断）。
func TestBestEffortVariantsSwallowErrors(t *testing.T) {
	t.Parallel()

	svc := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		&fakeVectorIndex{available: true, storeErr: errors.New("store failed"), deleteErr: errors.New("delete failed")},
		&fakeVectorSource{available: true, source: VideoVectorSource{ID: 9, Status: enum.VideoStatusPublished}}, zap.NewNop(),
	)

	svc.IndexVideoVectorBestEffort(context.Background(), 9)
	svc.DeleteVideoVectorBestEffort(context.Background(), 9)
}
