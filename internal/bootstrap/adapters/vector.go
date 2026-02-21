package adapters

import (
	"context"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"
	searchapp "f_ccg_video_back_ddd/internal/application/search"
	videoapp "f_ccg_video_back_ddd/internal/application/video"
)

// AIVideoVectorSearch 显式映射 AI 与 Search 上下文的多模态查询 DTO。
type AIVideoVectorSearch struct {
	vectors *searchapp.VideoVectorService
}

var _ aiapp.VideoVectorSearch = (*AIVideoVectorSearch)(nil)

// NewAIVideoVectorSearch 构造 AI → Search 查询适配器。
func NewAIVideoVectorSearch(vectors *searchapp.VideoVectorService) *AIVideoVectorSearch {
	return &AIVideoVectorSearch{vectors: vectors}
}

// SearchVideoVectorIDsMultiModal 执行跨上下文的多模态向量召回。
func (a *AIVideoVectorSearch) SearchVideoVectorIDsMultiModal(
	ctx context.Context,
	query aiapp.VideoVectorQuery,
	limit int,
) ([]uint, error) {
	return a.vectors.SearchVideoVectorIDsMultiModal(ctx, searchapp.EmbedInput{
		Text:     query.Text,
		ImageURL: query.ImageURL,
	}, limit)
}

// VideoVectorRebuilder 映射 video 与 search 上下文的回填参数。
type VideoVectorRebuilder struct {
	rebuild *searchapp.VideoVectorRebuildService
}

var _ videoapp.VectorIndexRebuilder = (*VideoVectorRebuilder)(nil)

// NewVideoVectorRebuilder 构造视频向量回填适配器。
func NewVideoVectorRebuilder(rebuild *searchapp.VideoVectorRebuildService) *VideoVectorRebuilder {
	return &VideoVectorRebuilder{rebuild: rebuild}
}

// StartRebuild 启动全量回填。
func (a *VideoVectorRebuilder) StartRebuild(options videoapp.VectorRebuildOptions) (bool, error) {
	return a.rebuild.StartRebuild(searchapp.RebuildOptions{
		Recreate: options.Recreate,
		Workers:  options.Workers,
	})
}

// RebuildStatus 返回当前回填状态。
func (a *VideoVectorRebuilder) RebuildStatus() any {
	return a.rebuild.RebuildStatus()
}
