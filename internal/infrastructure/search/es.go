// Package search 提供 Elasticsearch 客户端构建与关闭（原 core/init_es.go）。
package search

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"f_ccg_video_back_ddd/internal/infrastructure/config"

	elasticsearch8 "github.com/elastic/go-elasticsearch/v8"
	"github.com/olivere/elastic/v7"
	"go.uber.org/zap"
)

const (
	dialTimeout           = 10 * time.Second
	keepAlive             = 30 * time.Second
	maxIdleConns          = 100
	idleConnTimeout       = 90 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	expectContinueTimeout = 1 * time.Second
	pingTimeout           = 5 * time.Second
	pingRetries           = 5
	pingRetryInterval     = 3 * time.Second
)

// ES 聚合 v7 / v8 两个 Elasticsearch 客户端与自定义 HTTP 客户端。
//
// 配置缺失时全部句柄为 nil，与原实现一致（告警跳过初始化）。
type ES struct {
	v7         *elastic.Client
	v8         *elasticsearch8.Client
	httpClient *http.Client
	logger     *zap.Logger
}

// V7 返回 olivere/elastic v7 客户端（可能为 nil）。
func (e *ES) V7() *elastic.Client {
	if e == nil {
		return nil
	}

	return e.v7
}

// V8 返回官方 go-elasticsearch v8 客户端（可能为 nil）。
func (e *ES) V8() *elasticsearch8.Client {
	if e == nil {
		return nil
	}

	return e.v8
}

// New 连接 Elasticsearch（带 Ping 重试）；失败时 zap.Panic（与原 core.InitES 一致）。
func New(cfg *config.ES, logger *zap.Logger) *ES {
	if logger == nil {
		logger = zap.NewNop()
	}

	if cfg == nil || cfg.Host == "" || cfg.Port == 0 {
		logger.Warn("ES 配置为空，跳过初始化")

		return &ES{logger: logger}
	}

	scheme := "http"
	if cfg.IsHTTPS {
		scheme = "https"
	}

	url := fmt.Sprintf("%s://%s:%d", scheme, cfg.Host, cfg.Port)

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   dialTimeout, // 建立 TCP 连接超时
			KeepAlive: keepAlive,
		}).DialContext,
		MaxIdleConns:          maxIdleConns,
		IdleConnTimeout:       idleConnTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
	}

	if cfg.IsHTTPS && cfg.InsecureSkipVerify {
		logger.Warn("ES TLS certificate verification is disabled; use only in development")

		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // 仅开发环境显式开启
	}

	// 1. 自定义 HTTP Client
	httpClient := &http.Client{Transport: transport}

	// 2. 构造初始化参数
	opts := []elastic.ClientOptionFunc{
		elastic.SetURL(url),
		elastic.SetHttpClient(httpClient),
		// 关键：在 ES 8.x 或 Docker 环境下，强烈建议将以下两项设为 false
		elastic.SetSniff(false),       // 禁用嗅探，防止其获取容器内部无法访问的 IP
		elastic.SetHealthcheck(false), // 禁用启动时的强制健康检查，改由下文手动 Ping
		elastic.SetErrorLog(zap.NewStdLog(logger)),
	}

	// 填充账号密码
	if cfg.Username != "" && cfg.Password != "" {
		opts = append(opts, elastic.SetBasicAuth(cfg.Username, cfg.Password))
	}

	es8Config := elasticsearch8.Config{
		Addresses: []string{url},
		Username:  cfg.Username,
		Password:  cfg.Password,
		Transport: httpClient.Transport,
	}

	// 3. 初始化客户端
	client, err := elastic.NewClient(opts...)
	if err != nil {
		logger.Panic("ES 客户端创建失败 (配置错误)", zap.Error(err))
	}

	// 4. 手动测试连接 (带重试逻辑)
	// ES 8.x 启动后加载分词器（IK）需要时间，这里增加重试以提高稳定性
	// ES 初始化是进程启动 root 场景，没有请求上下文。
	ctx := context.Background()

	var (
		info    *elastic.PingResult
		code    int
		pingErr error
	)

	logger.Info("正在尝试连接 Elasticsearch...", zap.String("url", url))

	for i := range pingRetries {
		pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
		info, code, pingErr = client.Ping(url).Do(pingCtx)

		cancel()

		if pingErr == nil {
			break
		}

		logger.Warn("ES 尚未就绪或连接被拒绝，正在重试...",
			zap.Int("retry", i+1),
			zap.Error(pingErr))
		time.Sleep(pingRetryInterval)
	}

	if pingErr != nil {
		logger.Panic("ES 连接最终失败",
			zap.Error(pingErr),
			zap.String("url", url),
			zap.String("tips", "请检查 ES 是否开启了 HTTPS 但你使用了 http，或者账号密码错误"))
	}

	es8Client, err := elasticsearch8.NewClient(es8Config)
	if err != nil {
		logger.Panic("ES8 客户端创建失败 (配置错误)", zap.Error(err))
	}

	logger.Info("ES 连接成功",
		zap.String("cluster", info.ClusterName),
		zap.String("version", info.Version.Number),
		zap.Int("code", code))

	return &ES{v7: client, v8: es8Client, httpClient: httpClient, logger: logger}
}

// Stop 停止 ES v7 后台协程并释放 HTTP 空闲连接。
func (e *ES) Stop() {
	if e == nil {
		return
	}

	had := e.v7 != nil || e.v8 != nil || e.httpClient != nil

	if e.v7 != nil {
		e.v7.Stop()
		e.v7 = nil
	}

	// official v8 client 无显式 Close，置空句柄即可。
	e.v8 = nil

	if e.httpClient != nil {
		e.httpClient.CloseIdleConnections()
		e.httpClient = nil
	}

	if had {
		e.logger.Info("elasticsearch 连接已关闭")
	}
}

// ProvideV7 取出 v7 客户端供仓储注入（可能为 nil）。
func ProvideV7(e *ES) *elastic.Client {
	return e.V7()
}

// ProvideV8 取出 v8 客户端供仓储注入（可能为 nil）。
func ProvideV8(e *ES) *elasticsearch8.Client {
	return e.V8()
}

// Provide 供 wire 使用的 provider：返回 ES 客户端集合与其资源关闭函数。
func Provide(cfg *config.ES, logger *zap.Logger) (*ES, func()) {
	es := New(cfg, logger)

	return es, es.Stop
}
