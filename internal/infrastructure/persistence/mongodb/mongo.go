// Package mongodb 提供 Mongo 客户端构建与关闭。
package mongodb

import (
	"context"
	"time"

	"f_ccg_video_back_ddd/internal/infrastructure/config"

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
// Mongo 配置段或 db 缺失时，client 与 database 均为 nil；只告警并跳过初始化，相关功能在运行期降级。
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

// New 连接 Mongo 并 Ping 探活，失败时 zap.Panic。
//
// 聊天集合索引由 chat 上下文的 IndexInitializer 在后台组件启动时创建。
func New(cfg *config.Mongo, logger *zap.Logger) *Mongo {
	if cfg == nil {
		logger.Warn("Mongo 配置为空，跳过初始化")

		return &Mongo{logger: logger}
	}

	if cfg.DBName == "" {
		logger.Warn("Mongo db 配置为空，跳过初始化")

		return &Mongo{logger: logger}
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
		logger.Panic("Mongo 连接失败", zap.Error(err))
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		logger.Panic("Mongo Ping 失败", zap.Error(err))
	}

	logger.Info("Mongo 连接成功", zap.String("db", cfg.DBName))

	return &Mongo{client: client, database: client.Database(cfg.DBName), logger: logger}
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
		m.logger.Warn("Mongo Disconnect 失败", zap.Error(err))
	}

	m.client = nil
	m.database = nil

	m.logger.Info("Mongo 连接已关闭")
}

// ProvideDatabase 取出 *mongo.Database 供仓储注入（可能为 nil）。
func ProvideDatabase(m *Mongo) *mongo.Database {
	return m.Database()
}

// Provide 供 wire 使用的 provider：返回 Mongo 句柄与其资源关闭函数。
func Provide(cfg *config.Mongo, logger *zap.Logger) (*Mongo, func()) {
	m := New(cfg, logger)

	return m, m.Stop
}
