// Package mongodb 构造和关闭 MongoDB 客户端。
//
// mongo.go 创建连接并选择数据库，持久化子包负责 collection、索引与读写。
// 客户端关闭必须晚于私信处理和后台任务，避免在途请求使用已经断开的连接。
//
// provider.go 只导出构造器与窄化访问能力，不应在业务用例中再次创建全局连接。
package mongodb

import (
	"context"
	"f_ccg_video_back_ddd/internal/config"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/zap"
)

const (
	defaultConnectTimeout = 5 * time.Second
	disconnectTimeout     = 3 * time.Second
)

// Mongo 包装 Mongo 客户端与数据库句柄，负责连接生命周期。
//
// 配置缺失（Mongo 段为空或未配置 db）时 client/database 均为 nil，
// 与原实现一致：仅告警跳过初始化，依赖 Mongo 的功能在运行期各自降级。
type Mongo struct {
	client   *mongo.Client
	database *mongo.Database
	logger   *zap.Logger
}

// Client 返回底层 *mongo.Client（可能为 nil）。
func (m *Mongo) Client() *mongo.Client {
	if m == nil {
		return nil
	}

	return m.client
}

// Database 返回底层 *mongo.Database（可能为 nil）。
func (m *Mongo) Database() *mongo.Database {
	if m == nil {
		return nil
	}

	return m.database
}

// New 连接 Mongo 并 Ping 探活；失败返回错误，由入口记录并退出。
//
// 原实现在连接成功后顺带创建聊天集合索引；新架构把该职责下沉到 chat 上下文的
// 后台组件启动流程（见 internal/infrastructure/persistence/chat 索引初始化），
// 避免基础设施客户端反向依赖具体业务上下文。
func New(cfg *config.Mongo, logger *zap.Logger) (*Mongo, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	if cfg == nil {
		logger.Warn("Mongo 配置为空，跳过初始化", zap.String("event", "persistence.mongo.new.degraded"), zap.String("operation", "persistence.mongo.new"))

		return &Mongo{logger: logger}, nil
	}

	if cfg.DBName == "" {
		logger.Warn("Mongo db 配置为空，跳过初始化", zap.String("event", "persistence.mongo.new.degraded"), zap.String("operation", "persistence.mongo.new"))

		return &Mongo{logger: logger}, nil
	}

	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = defaultConnectTimeout
	}

	// Mongo 初始化是进程启动 root 场景，没有请求上下文。
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	opts := options.Client().ApplyURI(cfg.GetURI()).
		SetRetryWrites(true)
	if cfg.MaxPool > 0 {
		opts = opts.SetMaxPoolSize(cfg.MaxPool)
	}

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("ping mongo: %w", err)
	}

	logger.Info("Mongo 连接成功", zap.String("event", "persistence.mongo.new.diagnostic"), zap.String("operation", "persistence.mongo.new"), zap.String("db", cfg.DBName))

	return &Mongo{client: client, database: client.Database(cfg.DBName), logger: logger}, nil
}

// Stop 断开 Mongo 连接。
func (m *Mongo) Stop() {
	if m == nil || m.client == nil {
		return
	}

	// Mongo 关闭是进程退出 root 场景，使用独立超时。
	ctx, cancel := context.WithTimeout(context.Background(), disconnectTimeout)
	defer cancel()

	if err := m.client.Disconnect(ctx); err != nil {
		m.logger.Warn("Mongo Disconnect 失败", zap.String("event", "persistence.mongo.stop.degraded"), zap.String("operation", "persistence.mongo.stop"), zap.Error(err))
	}

	m.client = nil
	m.database = nil

	m.logger.Info("Mongo 连接已关闭", zap.String("event", "persistence.mongo.stop.diagnostic"), zap.String("operation", "persistence.mongo.stop"))
}

// ProvideDatabase 取出 *mongo.Database 供仓储注入（可能为 nil）。
func ProvideDatabase(m *Mongo) *mongo.Database {
	return m.Database()
}

// Provide 供 wire 使用的 provider：返回 Mongo 句柄与其资源关闭函数。
func Provide(cfg *config.Mongo, logger *zap.Logger) (*Mongo, func(), error) {
	m, err := New(cfg, logger)
	if err != nil {
		return nil, nil, err
	}

	return m, m.Stop, nil
}
