package chat

import (
	"context"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/chat"

	"go.uber.org/zap"
)

// indexInitTimeout 索引初始化独立超时。
const indexInitTimeout = 10 * time.Second

// IndexInitializer 是启动期创建聊天集合索引的后台组件，由 bootstrap 纳入 App 生命周期。
type IndexInitializer struct {
	repo   domain.Repository
	logger *zap.Logger
}

// NewIndexInitializer 构造索引初始化组件。
func NewIndexInitializer(repo domain.Repository, logger *zap.Logger) *IndexInitializer {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &IndexInitializer{repo: repo, logger: logger}
}

// Start 使用独立 10s 超时创建聊天集合索引。
//
// Mongo 缺失时跳过，创建失败只告警、不阻断启动，因此始终返回 nil。
func (i *IndexInitializer) Start(_ context.Context) error {
	if i == nil || i.repo == nil || !i.repo.Available() {
		return nil
	}

	// Mongo 索引初始化是启动期后台维护任务，使用独立 root 超时。
	indexCtx, cancel := context.WithTimeout(context.Background(), indexInitTimeout)
	defer cancel()

	if err := i.repo.EnsureIndexes(indexCtx); err != nil {
		i.logger.Warn("Mongo 索引初始化失败", zap.Error(err))
	}

	return nil
}

// Stop 无需释放资源（索引创建是一次性启动动作），始终返回 nil。
func (i *IndexInitializer) Stop(_ context.Context) error {
	return nil
}
