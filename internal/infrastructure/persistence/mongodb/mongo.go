// Package mongodb 提供 Mongo 客户端构建与关闭（原 core/init_mongo.go）。
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
// 配置缺失（Mongo 段为空或未配置 db）时 client/database 均为 nil，
// 与原实现一致：仅告警跳过初始化，依赖 Mongo 的功能在运行期各自降级。
type Mongo struct {
	client   *mongo.Client
	database *mongo.Database
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

// New 连接 Mongo 并 Ping 探活；失败时 zap.Panic（与原 core.InitMongo 一致）。
//
// 原实现在连接成功后顺带创建聊天集合索引；新架构把该职责下沉到 chat 上下文的
// 后台组件启动流程（见 internal/infrastructure/persistence/chat 索引初始化），
// 避免基础设施客户端反向依赖具体业务上下文。
func New(cfg *config.Mongo) *Mongo {
	if cfg == nil {
		zap.L().Warn("Mongo 配置为空，跳过初始化")

		return &Mongo{}
	}

	if cfg.DBName == "" {
		zap.L().Warn("Mongo db 配置为空，跳过初始化")

		return &Mongo{}
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
		zap.L().Panic("Mongo 连接失败", zap.Error(err))
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		zap.L().Panic("Mongo Ping 失败", zap.Error(err))
	}

	zap.L().Info("Mongo 连接成功", zap.String("db", cfg.DBName))

	return &Mongo{client: client, database: client.Database(cfg.DBName)}
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
		zap.L().Warn("Mongo Disconnect 失败", zap.Error(err))
	}

	m.client = nil
	m.database = nil

	zap.L().Info("Mongo 连接已关闭")
}

// ProvideDatabase 取出 *mongo.Database 供仓储注入（可能为 nil）。
func ProvideDatabase(m *Mongo) *mongo.Database {
	return m.Database()
}

// Provide 供 wire 使用的 provider：返回 Mongo 句柄与其资源关闭函数。
func Provide(cfg *config.Mongo) (*Mongo, func()) {
	m := New(cfg)

	return m, m.Stop
}
