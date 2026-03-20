package chat

import (
	"context"
	"time"

	chatrepo "f_ccg_video_back_ddd/internal/domain/chat/repository"

	"go.uber.org/zap"
)

// indexInitTimeout 索引初始化独立超时。
const indexInitTimeout = 10 * time.Second

// IndexInitializer 在服务启动时创建聊天集合索引。
//
// 由 bootstrap 纳入 App 启动序列，避免数据库客户端依赖业务上下文。
type IndexInitializer struct {
	repo   chatrepo.Repository
	logger *zap.Logger
}

// NewIndexInitializer 构造索引初始化组件。
func NewIndexInitializer(repo chatrepo.Repository, logger *zap.Logger) *IndexInitializer {
	return &IndexInitializer{repo: repo, logger: logger}
}

// Start 创建聊天集合索引。
//
// 使用独立 10s 超时；Mongo 未初始化时跳过；创建失败仅告警，不阻断启动。
func (i *IndexInitializer) Start(_ context.Context) {
	if i == nil || i.repo == nil || !i.repo.Available() {
		return
	}

	// Mongo 索引初始化是启动期后台维护任务，使用独立 root 超时。
	indexCtx, cancel := context.WithTimeout(context.Background(), indexInitTimeout)
	defer cancel()

	if err := i.repo.EnsureIndexes(indexCtx); err != nil {
		i.logger.Warn("Mongo 索引初始化失败", zap.Error(err))
	}
}
