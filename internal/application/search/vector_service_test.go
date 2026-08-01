package search

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/internal/infrastructure/config/site"

	"go.uber.org/zap"
)

// 对应原 service/search_service/video_vector.go（原文件无单测，此处按分支补齐）。

// newVectorConfigProvider 造一份可用的 AI 配置（向量检索的必填项齐备）。
func newVectorConfigProvider(ai *config.AI) *config.Provider {
	return config.NewProvider(&config.Config{AI: ai})
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
		&fakeVectorSource{available: true}, zap.NewNop())

	cfg, err := svc.vectorConfig()
	if err != nil {
		t.Fatalf("vectorConfig: %v", err)
	}

	if cfg.Index != defaultVideoVectorIndex {
		t.Fatalf("index = %q, want %q", cfg.Index, defaultVideoVectorIndex)
	}

	if cfg.TopK != defaultVideoVectorTopK {
		t.Fatalf("topK = %d, want %d", cfg.TopK, defaultVideoVectorTopK)
	}

	if cfg.EmbeddingDimensions != defaultVideoEmbeddingDims {
		t.Fatalf("dims = %d, want %d", cfg.EmbeddingDimensions, defaultVideoEmbeddingDims)
	}

	if cfg.ImageMode != embeddingImageModeAuto || cfg.RRFRankConstant != defaultVectorRRFRankConstant {
		t.Fatalf("defaults = image:%q rrf:%d", cfg.ImageMode, cfg.RRFRankConstant)
	}
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

	provider := config.NewProvider(&config.Config{
		AI: ai,
		Site: &site.Site{Storage: &site.Storage{Local: &site.Local{
			Enable: true,
			Path:   " uploads ",
		}}},
	})
	svc := NewVideoVectorService(provider, &fakeVectorIndex{}, &fakeVectorSource{}, zap.NewNop())

	cfg, err := svc.vectorConfig()
	if err != nil {
		t.Fatalf("vectorConfig: %v", err)
	}

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
	if cfg != want {
		t.Fatalf("cfg = %+v, want %+v", cfg, want)
	}
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

			svc := NewVideoVectorService(config.NewProvider(tt.cfg), &fakeVectorIndex{}, &fakeVectorSource{}, zap.NewNop())
			if _, err := svc.vectorConfig(); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
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
	if err != nil {
		t.Fatalf("SearchVideoVectorIDs: %v", err)
	}

	if ids != nil {
		t.Fatalf("ids = %v, want nil", ids)
	}

	if len(index.searchCalls) != 0 {
		t.Fatalf("blank query must not reach the index: %v", index.searchCalls)
	}
}

func TestSearchVideoVectorIDsRequiresES8(t *testing.T) {
	t.Parallel()

	svc := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		&fakeVectorIndex{available: false},
		&fakeVectorSource{available: true}, zap.NewNop())

	if _, err := svc.SearchVideoVectorIDs(context.Background(), "教程", 5); !errors.Is(err, ErrES8Unavailable) {
		t.Fatalf("err = %v, want ErrES8Unavailable", err)
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
	if err != nil {
		t.Fatalf("SearchVideoVectorIDs: %v", err)
	}

	if !reflect.DeepEqual(ids, []uint{1, 2}) {
		t.Fatalf("ids = %v", ids)
	}

	if index.searchLimit != 3 {
		t.Fatalf("limit = %d, want 3 (clamped to TopK)", index.searchLimit)
	}

	// query 传给索引前已 TrimSpace。
	if !reflect.DeepEqual(index.searchCalls, []EmbedInput{{Text: "教程"}}) {
		t.Fatalf("queries = %v", index.searchCalls)
	}

	// limit<=0 时直接使用配置 TopK。
	if _, err := svc.SearchVideoVectorIDs(context.Background(), "教程", 0); err != nil {
		t.Fatalf("SearchVideoVectorIDs: %v", err)
	}

	if index.searchLimit != 3 {
		t.Fatalf("default limit = %d, want 3", index.searchLimit)
	}
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

	if _, err := svc.SearchVideoVectorIDs(context.Background(), "教程", 0); err != nil {
		t.Fatalf("SearchVideoVectorIDs: %v", err)
	}
	if index.searchLimit != 12 {
		t.Fatalf("limit = %d, want configured TopK 12", index.searchLimit)
	}
}

func TestNormalizeEmbeddingImageMode(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"":        embeddingImageModeAuto,
		"AUTO":    embeddingImageModeAuto,
		" on ":    embeddingImageModeOn,
		"off":     embeddingImageModeOff,
		"unknown": embeddingImageModeAuto,
	}
	for input, want := range tests {
		if got := normalizeEmbeddingImageMode(input); got != want {
			t.Fatalf("normalizeEmbeddingImageMode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSearchVideoVectorIDsMultiModalForwardsImage(t *testing.T) {
	t.Parallel()

	index := &fakeVectorIndex{available: true, ids: []uint{7}}
	svc := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		index,
		&fakeVectorSource{available: true}, zap.NewNop())

	ids, err := svc.SearchVideoVectorIDsMultiModal(context.Background(), EmbedInput{
		Text:     "  红色跑车 ",
		ImageURL: " https://cdn.example.com/query.jpg ",
	}, 2)
	if err != nil {
		t.Fatalf("SearchVideoVectorIDsMultiModal: %v", err)
	}
	if !reflect.DeepEqual(ids, []uint{7}) {
		t.Fatalf("ids = %v", ids)
	}

	want := []EmbedInput{{Text: "红色跑车", ImageURL: "https://cdn.example.com/query.jpg"}}
	if !reflect.DeepEqual(index.searchCalls, want) {
		t.Fatalf("inputs = %+v, want %+v", index.searchCalls, want)
	}
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
			if err := svc.IndexVideoVector(context.Background(), tt.videoID); err != nil {
				t.Fatalf("IndexVideoVector: %v", err)
			}

			if len(tt.index.stored) != 0 || len(tt.index.deleted) != 0 {
				t.Fatalf("index touched: stored=%v deleted=%v", tt.index.stored, tt.index.deleted)
			}
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
		&fakeVectorSource{available: true, source: source}, zap.NewNop())

	if err := svc.IndexVideoVector(context.Background(), 9); err != nil {
		t.Fatalf("IndexVideoVector: %v", err)
	}

	if !reflect.DeepEqual(index.stored, []VideoVectorSource{source}) {
		t.Fatalf("stored = %+v", index.stored)
	}

	if len(index.deleted) != 0 {
		t.Fatalf("published video must not be deleted: %v", index.deleted)
	}
}

// TestIndexVideoVectorDeletesWhenNotPublished 非「已发布」状态改为删除文档
// （避免下架/私密视频仍被向量召回）。
func TestIndexVideoVectorDeletesWhenNotPublished(t *testing.T) {
	t.Parallel()

	index := &fakeVectorIndex{available: true}
	svc := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		index,
		&fakeVectorSource{available: true, source: VideoVectorSource{ID: 9, Status: enum.VideoStatusPrivate}}, zap.NewNop())

	if err := svc.IndexVideoVector(context.Background(), 9); err != nil {
		t.Fatalf("IndexVideoVector: %v", err)
	}

	if !reflect.DeepEqual(index.deleted, []uint{9}) {
		t.Fatalf("deleted = %v, want [9]", index.deleted)
	}

	if len(index.stored) != 0 {
		t.Fatalf("unpublished video must not be stored: %+v", index.stored)
	}
}

// TestIndexVideoVectorDeletesWhenSourceMissing 源视频已被物理删除时删除向量文档。
func TestIndexVideoVectorDeletesWhenSourceMissing(t *testing.T) {
	t.Parallel()

	index := &fakeVectorIndex{available: true}
	svc := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		index,
		&fakeVectorSource{available: true, err: ErrVectorSourceNotFound}, zap.NewNop())

	if err := svc.IndexVideoVector(context.Background(), 9); err != nil {
		t.Fatalf("IndexVideoVector: %v", err)
	}

	if !reflect.DeepEqual(index.deleted, []uint{9}) {
		t.Fatalf("deleted = %v, want [9]", index.deleted)
	}
}

func TestIndexVideoVectorPropagatesSourceError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("db down")
	svc := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		&fakeVectorIndex{available: true},
		&fakeVectorSource{available: true, err: wantErr}, zap.NewNop())

	if err := svc.IndexVideoVector(context.Background(), 9); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

// TestIndexVideoVectorPropagatesConfigError 配置不全时报错，且不触碰索引。
func TestIndexVideoVectorPropagatesConfigError(t *testing.T) {
	t.Parallel()

	index := &fakeVectorIndex{available: true}
	svc := NewVideoVectorService(
		config.NewProvider(&config.Config{}),
		index,
		&fakeVectorSource{available: true}, zap.NewNop())

	if err := svc.IndexVideoVector(context.Background(), 9); !errors.Is(err, ErrAIConfigMissing) {
		t.Fatalf("err = %v, want ErrAIConfigMissing", err)
	}

	if len(index.stored) != 0 || len(index.deleted) != 0 {
		t.Fatalf("index touched: stored=%v deleted=%v", index.stored, index.deleted)
	}
}

// TestDeleteVideoVectorSkipsWhenUnavailable videoID 为 0 或 ES8 缺失时静默跳过。
func TestDeleteVideoVectorSkipsWhenUnavailable(t *testing.T) {
	t.Parallel()

	index := &fakeVectorIndex{available: true}
	svc := NewVideoVectorService(newVectorConfigProvider(validVectorAIConfig()), index, &fakeVectorSource{}, zap.NewNop())

	if err := svc.DeleteVideoVector(context.Background(), 0); err != nil {
		t.Fatalf("DeleteVideoVector: %v", err)
	}

	offline := &fakeVectorIndex{available: false}
	svc = NewVideoVectorService(newVectorConfigProvider(validVectorAIConfig()), offline, &fakeVectorSource{}, zap.NewNop())

	if err := svc.DeleteVideoVector(context.Background(), 9); err != nil {
		t.Fatalf("DeleteVideoVector: %v", err)
	}

	if len(index.deleted) != 0 || len(offline.deleted) != 0 {
		t.Fatalf("delete must be skipped: %v %v", index.deleted, offline.deleted)
	}
}

func TestDeleteVideoVectorForwardsToIndex(t *testing.T) {
	t.Parallel()

	index := &fakeVectorIndex{available: true}
	svc := NewVideoVectorService(newVectorConfigProvider(validVectorAIConfig()), index, &fakeVectorSource{}, zap.NewNop())

	if err := svc.DeleteVideoVector(context.Background(), 9); err != nil {
		t.Fatalf("DeleteVideoVector: %v", err)
	}

	if !reflect.DeepEqual(index.deleted, []uint{9}) {
		t.Fatalf("deleted = %v, want [9]", index.deleted)
	}
}

// TestBestEffortVariantsSwallowErrors BestEffort 版本只记日志，不向调用方传播错误
// （视频发布/下架流程不能因向量同步失败而中断）。
func TestBestEffortVariantsSwallowErrors(t *testing.T) {
	t.Parallel()

	svc := NewVideoVectorService(
		newVectorConfigProvider(validVectorAIConfig()),
		&fakeVectorIndex{available: true, storeErr: errors.New("store failed"), deleteErr: errors.New("delete failed")},
		&fakeVectorSource{available: true, source: VideoVectorSource{ID: 9, Status: enum.VideoStatusPublished}}, zap.NewNop())

	svc.IndexVideoVectorBestEffort(context.Background(), 9)
	svc.DeleteVideoVectorBestEffort(context.Background(), 9)
}
