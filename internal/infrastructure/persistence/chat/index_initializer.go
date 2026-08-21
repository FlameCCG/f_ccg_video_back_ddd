package chat

import (
	"context"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/chat"

	"go.uber.org/zap"
)

// indexInitTimeout 索引初始化独立超时（与原 core.InitMongo 尾段一致）。
const indexInitTimeout = 10 * time.Second

// IndexInitializer 启动期创建聊天集合索引的后台组件。
//
// 原实现把该动作硬编码在 core.InitMongo 连接成功之后（直接调用
// chat_service.ChatService{}.EnsureIndexes）；新架构改为可注入的生命周期组件，
// 由 bootstrap 挂进 App 启动序列，避免基础设施客户端反向依赖业务上下文。
type IndexInitializer struct {
	repo   domain.Repository
	logger *zap.Logger
}

// NewIndexInitializer 构造索引初始化组件。
func NewIndexInitializer(repo domain.Repository, logger *zap.Logger) *IndexInitializer {
	return &IndexInitializer{repo: repo, logger: logger}
}

// Start 创建聊天集合索引。
//
// 语义与原实现一致：使用独立 10s 超时；Mongo 未初始化时静默跳过
// （原实现此时根本不会走到 EnsureIndexes）；创建失败仅告警、不阻断启动，
// 因此始终返回 nil。
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
