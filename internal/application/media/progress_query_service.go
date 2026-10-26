package media

import (
	"context"
	"sync"

	domain "f_ccg_video_back_ddd/internal/domain/media"
)

// ProgressQueryService 转码进度查询与订阅用例
// （原 mq_service.GetTranscodeProgress / MGetTranscodeProgressByVideoIDs /
// MGetTranscodeProgress / SubscribeTranscodeProgress）。
//
// 管理端进度接口与 SSE 经此服务读取 Redis 快照；Redis key 与 TTL 由 cache/media 适配器保证 1:1。
type ProgressQueryService struct {
	reader domain.ProgressReader
}

// NewProgressQueryService 构造进度查询服务。
func NewProgressQueryService(reader domain.ProgressReader) *ProgressQueryService {
	return &ProgressQueryService{reader: reader}
}

// Get 读取单条进度；未命中时返回 (nil, nil)。
func (s *ProgressQueryService) Get(ctx context.Context, videoID, partID uint) (*TranscodeProgressItem, error) {
	snapshot, err := s.reader.Get(ctx, videoID, partID)
	if err != nil || snapshot == nil {
		return nil, err
	}

	item := progressItemFrom(*snapshot)

	return &item, nil
}

// MGetByVideoIDs 批量读取给定视频的进度快照（存在分P 进度时展开为各分P）。
func (s *ProgressQueryService) MGetByVideoIDs(ctx context.Context, videoIDs []uint) ([]TranscodeProgressItem, error) {
	snapshots, err := s.reader.ByVideoIDs(ctx, videoIDs)
	if err != nil {
		return nil, err
	}

	return progressItemsFrom(snapshots), nil
}

// IsTerminalStatus 判断进度状态是否为终态（succeeded / failed）。
func (s *ProgressQueryService) IsTerminalStatus(status string) bool {
	return status == domain.TranscodeStatusSucceeded || status == domain.TranscodeStatusFailed
}

// Subscribe 订阅进度变更事件；返回的订阅需由调用方 Close。
func (s *ProgressQueryService) Subscribe(ctx context.Context) (*ProgressSubscription, error) {
	sub, err := s.reader.Subscribe(ctx)
	if err != nil {
		return nil, err
	}

	return newProgressSubscription(ctx, sub), nil
}

// ProgressSubscription 是进度变更订阅句柄，把领域快照映射为读模型事件。
type ProgressSubscription struct {
	ctx       context.Context
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
	inner     domain.ProgressSubscription
	events    chan TranscodeProgressItem
}

// newProgressSubscription 启动映射 goroutine 并返回订阅句柄。
func newProgressSubscription(ctx context.Context, inner domain.ProgressSubscription) *ProgressSubscription {
	s := &ProgressSubscription{
		ctx:    ctx,
		done:   make(chan struct{}),
		inner:  inner,
		events: make(chan TranscodeProgressItem, 32),
	}

	go s.pump()

	return s
}

// pump 把领域事件通道转成读模型事件通道；上游关闭时随之关闭。
func (s *ProgressSubscription) pump() {
	defer close(s.events)
	defer s.Close()
	input := s.inner.Events()
	for {
		select {
		case <-s.done:
			return
		case <-s.ctx.Done():
			return
		case snapshot, ok := <-input:
			if !ok {
				return
			}

			select {
			case <-s.done:
				return
			case <-s.ctx.Done():
				return
			case s.events <- progressItemFrom(snapshot):
			}
		}
	}
}

// Events 返回进度事件通道；通道关闭表示订阅结束。
func (s *ProgressSubscription) Events() <-chan TranscodeProgressItem {
	return s.events
}

// Close 关闭底层订阅（pump goroutine 随之退出）。
func (s *ProgressSubscription) Close() error {
	s.closeOnce.Do(func() { close(s.done); s.closeErr = s.inner.Close() })
	return s.closeErr
}

// progressItemsFrom 批量转换进度快照。
func progressItemsFrom(snapshots []domain.TranscodeProgress) []TranscodeProgressItem {
	out := make([]TranscodeProgressItem, 0, len(snapshots))
	for _, snapshot := range snapshots {
		out = append(out, progressItemFrom(snapshot))
	}

	return out
}
