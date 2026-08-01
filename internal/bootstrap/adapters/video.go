package adapters

import (
	"context"

	mediaapp "f_ccg_video_back_ddd/internal/application/media"
	searchapp "f_ccg_video_back_ddd/internal/application/search"
	videoapp "f_ccg_video_back_ddd/internal/application/video"
	mediadomain "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// VideoStorageResolver 把 media 存储驱动接口窄化为 video 上下文所需的方法集。
type VideoStorageResolver struct {
	resolver mediadomain.StorageResolver
}

var _ videoapp.StorageResolver = (*VideoStorageResolver)(nil)

// NewVideoStorageResolver 构造视频存储适配器。
func NewVideoStorageResolver(resolver mediadomain.StorageResolver) *VideoStorageResolver {
	return &VideoStorageResolver{resolver: resolver}
}

// Resolve 解析当前存储驱动。
func (a *VideoStorageResolver) Resolve() (videoapp.StorageDriver, error) {
	driver, err := a.resolver.Resolve()
	if err != nil {
		return nil, err
	}

	return driver, nil
}

// ResolveByType 按视频记录的存储类型解析驱动。
func (a *VideoStorageResolver) ResolveByType(
	storageType enum.VideoStorageType,
) (videoapp.StorageDriver, error) {
	driver, err := a.resolver.ResolveByType(storageType)
	if err != nil {
		return nil, err
	}

	return driver, nil
}

// ValidateUploadID 校验上传标识。
func (a *VideoStorageResolver) ValidateUploadID(id string) error {
	return a.resolver.ValidateUploadID(id)
}

// ValidateChunkIndex 校验分片索引。
func (a *VideoStorageResolver) ValidateChunkIndex(index string) error {
	return a.resolver.ValidateChunkIndex(index)
}

// VideoTranscodeDispatcher 映射 video 与 media 的转码任务 DTO。
type VideoTranscodeDispatcher struct {
	dispatch *mediaapp.TranscodeDispatchService
}

var _ videoapp.TranscodeDispatcher = (*VideoTranscodeDispatcher)(nil)

// NewVideoTranscodeDispatcher 构造转码任务适配器。
func NewVideoTranscodeDispatcher(dispatch *mediaapp.TranscodeDispatchService) *VideoTranscodeDispatcher {
	return &VideoTranscodeDispatcher{dispatch: dispatch}
}

// PublishTranscodeTask 投递转码任务。
func (a *VideoTranscodeDispatcher) PublishTranscodeTask(
	ctx context.Context,
	task videoapp.TranscodeTask,
) error {
	return a.dispatch.PublishTranscodeTask(ctx, mediaapp.TranscodeTaskInput{
		ResolutionList: append([]uint16(nil), task.ResolutionList...),
		StorageType:    task.StorageType,
		SourcePath:     task.SourcePath,
		OutputPath:     task.OutputPath,
		FileHash:       task.FileHash,
		FileName:       task.FileName,
		VideoID:        task.VideoID,
		VideoPartID:    task.VideoPartID,
	})
}

// MarkTranscodeQueued 标记任务已入队。
func (a *VideoTranscodeDispatcher) MarkTranscodeQueued(ctx context.Context, videoID, partID uint) {
	a.dispatch.MarkTranscodeQueued(ctx, videoID, partID)
}

// VideoTranscodeProgressReader 映射 media 与 video 的转码进度 DTO。
type VideoTranscodeProgressReader struct {
	progress *mediaapp.ProgressQueryService
}

var _ videoapp.TranscodeProgressReader = (*VideoTranscodeProgressReader)(nil)

// NewVideoTranscodeProgressReader 构造转码进度适配器。
func NewVideoTranscodeProgressReader(
	progress *mediaapp.ProgressQueryService,
) *VideoTranscodeProgressReader {
	return &VideoTranscodeProgressReader{progress: progress}
}

// MGetByVideoIDs 批量读取视频转码进度。
func (a *VideoTranscodeProgressReader) MGetByVideoIDs(
	ctx context.Context,
	videoIDs []uint,
) ([]videoapp.TranscodeProgressItem, error) {
	items, err := a.progress.MGetByVideoIDs(ctx, videoIDs)
	if err != nil {
		return nil, err
	}

	return videoProgressItems(items), nil
}

// Subscribe 订阅转码进度。
func (a *VideoTranscodeProgressReader) Subscribe(
	ctx context.Context,
) (videoapp.TranscodeProgressSubscription, error) {
	subscription, err := a.progress.Subscribe(ctx)
	if err != nil {
		return nil, err
	}

	return newVideoProgressSubscription(subscription), nil
}

// IsTerminalStatus 判断进度是否处于终态。
func (a *VideoTranscodeProgressReader) IsTerminalStatus(status string) bool {
	return a.progress.IsTerminalStatus(status)
}

type videoProgressSubscription struct {
	inner  *mediaapp.ProgressSubscription
	events chan videoapp.TranscodeProgressItem
}

func newVideoProgressSubscription(inner *mediaapp.ProgressSubscription) *videoProgressSubscription {
	subscription := &videoProgressSubscription{
		inner:  inner,
		events: make(chan videoapp.TranscodeProgressItem, 32),
	}
	go subscription.pump()

	return subscription
}

func (s *videoProgressSubscription) pump() {
	defer close(s.events)

	for item := range s.inner.Events() {
		s.events <- videoProgressItem(item)
	}
}

func (s *videoProgressSubscription) Events() <-chan videoapp.TranscodeProgressItem {
	return s.events
}

func (s *videoProgressSubscription) Close() error {
	return s.inner.Close()
}

func videoProgressItems(items []mediaapp.TranscodeProgressItem) []videoapp.TranscodeProgressItem {
	result := make([]videoapp.TranscodeProgressItem, 0, len(items))
	for _, item := range items {
		result = append(result, videoProgressItem(item))
	}

	return result
}

func videoProgressItem(item mediaapp.TranscodeProgressItem) videoapp.TranscodeProgressItem {
	pipelines := make([]videoapp.TranscodePipelineProgress, 0, len(item.Pipelines))
	for _, pipeline := range item.Pipelines {
		pipelines = append(pipelines, videoapp.TranscodePipelineProgress{
			Name:        pipeline.Name,
			Status:      pipeline.Status,
			Percent:     pipeline.Percent,
			Message:     pipeline.Message,
			Error:       pipeline.Error,
			Resolutions: append([]uint16(nil), pipeline.Resolutions...),
			UpdatedAt:   pipeline.UpdatedAt,
		})
	}

	return videoapp.TranscodeProgressItem{
		VideoID:      item.VideoID,
		PartID:       item.PartID,
		Status:       item.Status,
		Percent:      item.Percent,
		Stage:        item.Stage,
		StagePercent: item.StagePercent,
		Message:      item.Message,
		DurationSec:  item.DurationSec,
		OutTimeSec:   item.OutTimeSec,
		Error:        item.Error,
		UpdatedAt:    item.UpdatedAt,
		StartedAt:    item.StartedAt,
		Pipelines:    pipelines,
	}
}

// VideoSearcher 组合 search 上下文的搜索、联想和热搜服务。
type VideoSearcher struct {
	search      *searchapp.SearchService
	suggestions *searchapp.SuggestService
	hot         *searchapp.HotKeywordService
}

var _ videoapp.VideoSearcher = (*VideoSearcher)(nil)

// NewVideoSearcher 构造视频侧搜索适配器。
func NewVideoSearcher(
	search *searchapp.SearchService,
	suggestions *searchapp.SuggestService,
	hot *searchapp.HotKeywordService,
) *VideoSearcher {
	return &VideoSearcher{search: search, suggestions: suggestions, hot: hot}
}

// SearchVideos 搜索视频。
func (a *VideoSearcher) SearchVideos(
	ctx context.Context,
	options videoapp.VideoSearchOptions,
) (any, error) {
	return a.search.SearchVideos(ctx, searchapp.SearchOptions{
		Keyword:    options.Keyword,
		Page:       options.Page,
		PageSize:   options.PageSize,
		VideoSort:  options.VideoSort,
		VideoOrder: options.VideoOrder,
	})
}

// Suggest 返回搜索联想。
func (a *VideoSearcher) Suggest(ctx context.Context, prefix string, size int) (any, error) {
	return a.suggestions.Suggest(ctx, prefix, size)
}

// HotKeywords 返回热搜词。
func (a *VideoSearcher) HotKeywords(
	ctx context.Context,
	page, pageSize int,
) (any, int64, error) {
	return a.hot.HotKeywords(ctx, page, pageSize)
}

// RecordHotKeyword 记录热搜词分值。
func (a *VideoSearcher) RecordHotKeyword(ctx context.Context, keyword string) {
	a.hot.RecordHotKeyword(ctx, keyword)
}
