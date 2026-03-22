package video

import (
	"context"

	videodomain "f_ccg_video_back_ddd/internal/domain/video"

	"go.uber.org/zap"
)

// HotRankDecayService 热门榜时间衰减用例。
type HotRankDecayService struct {
	store  HotRankStore
	policy videodomain.HotRankDecayPolicy
	logger *zap.Logger
}

// NewHotRankDecayService 构造热门榜衰减用例服务。
func NewHotRankDecayService(store HotRankStore, logger *zap.Logger) *HotRankDecayService {
	return &HotRankDecayService{store: store, policy: videodomain.NewHotRankDecayPolicy(), logger: logger}
}

// Run 对热门榜单做一轮时间衰减，避免老视频长期占榜。
//
// 步骤读榜单大小 → 读全部成员分值 → 管线内衰减/移除
// → 再读大小 → 超容量时按分值从低到高裁剪。
// 每一步失败只记录警告日志；衰减管线失败不阻断后续裁剪。
func (s *HotRankDecayService) Run(ctx context.Context) {
	if !s.store.Available() {
		s.logger.Warn("skip DecayVideoHotScore: redis not configured")

		return
	}

	// Cron 任务是调度 root 场景，没有请求上下文。
	if ctx == nil {
		ctx = context.Background()
	}

	total, err := s.store.Size(ctx)
	if err != nil {
		s.logger.Warn("fetch hot rank size failed", zap.Error(err))

		return
	}

	if total == 0 {
		return
	}

	entries, err := s.store.Entries(ctx)
	if err != nil {
		s.logger.Warn("fetch hot rank scores failed", zap.Error(err))

		return
	}

	if len(entries) == 0 {
		return
	}

	if err := s.store.ApplyDecay(ctx, s.policy.DecayAll(entries)); err != nil {
		s.logger.Warn("decay hot rank failed", zap.Error(err))
	}

	total, err = s.store.Size(ctx)
	if err != nil {
		s.logger.Warn("fetch hot rank size failed", zap.Error(err))

		return
	}

	if removeCount := s.policy.OverflowCount(total); removeCount > 0 {
		if err := s.store.TrimLowest(ctx, removeCount); err != nil {
			s.logger.Warn("trim hot rank failed", zap.Error(err))
		}
	}
}
