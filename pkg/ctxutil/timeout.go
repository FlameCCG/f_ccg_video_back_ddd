// Package ctxutil provides package functionality.
package ctxutil

import (
	"context"
	"time"
)

const (
	// RequestFastTimeout 用于轻量打点、热词等极短请求。
	RequestFastTimeout = 2 * time.Second
	// RequestDefaultTimeout 用于普通查询和互动类 HTTP API。
	RequestDefaultTimeout = 3 * time.Second
	// RequestSlowTimeout 用于慢查询和后台管理写操作。
	RequestSlowTimeout = 5 * time.Second
	// RequestHeavyTimeout 用于重操作和批量删除。
	RequestHeavyTimeout = 30 * time.Second
	// RequestLongTimeout 用于发布、较重但仍需同步响应的操作。
	RequestLongTimeout = 15 * time.Second
	// RequestOAuthTimeout 用于第三方 OAuth 登录回调链路。
	RequestOAuthTimeout = 30 * time.Second
	// RequestAITimeout 用于 AI 生成和流式响应入口。
	RequestAITimeout = 90 * time.Second
	// RequestTranscodeProgressSSETimeout 管理端转码进度 SSE 长连接窗口（断线后 EventSource 可重连）。
	RequestTranscodeProgressSSETimeout = 60 * time.Minute
	// RequestUploadTimeout 用于上传分片和合并，避免大文件上传被普通 API 超时误杀。
	RequestUploadTimeout = 10 * time.Minute

	// AsyncQuickTimeout 常量.
	AsyncQuickTimeout = 2 * time.Second
	// AsyncDefaultTimeout 常量.
	AsyncDefaultTimeout = 3 * time.Second
	// AsyncLongTimeout 及相关导出常量.
	// AsyncLongTimeout 常量.
	AsyncLongTimeout = 15 * time.Second

	// ExternalShortTimeout 用于公网 IP 等短外部 HTTP 探测。
	ExternalShortTimeout = 3 * time.Second
	// ExternalHeadTimeout 用于远程资源 HEAD 元信息探测。
	ExternalHeadTimeout = 10 * time.Second

	// RedisDialTimeout 用于 Redis 建连阶段。
	RedisDialTimeout = 5 * time.Second
	// RedisReadTimeout 用于 Redis 读操作。
	RedisReadTimeout = 3 * time.Second
	// RedisWriteTimeout 用于 Redis 写操作。
	RedisWriteTimeout = 3 * time.Second
	// RedisPoolTimeout 用于等待 Redis 连接池可用连接。
	RedisPoolTimeout = 4 * time.Second
	// RedisPingTimeout 用于启动阶段 Redis Ping 健康检查。
	RedisPingTimeout = 3 * time.Second

	// HTTPReadHeaderTimeout 用于限制请求头读取时间，避免慢速头攻击。
	HTTPReadHeaderTimeout = 5 * time.Second
	// HTTPReadTimeout 用于限制完整请求读取时间，需覆盖大文件上传场景。
	HTTPReadTimeout = RequestUploadTimeout + 30*time.Second
	// HTTPWriteTimeout 用于限制响应写出时间，避免慢响应长期占用连接。
	HTTPWriteTimeout = RequestUploadTimeout + 30*time.Second
	// HTTPIdleTimeout 用于空闲连接保活上限。
	HTTPIdleTimeout = 60 * time.Second
	// HTTPShutdownTimeout 用于优雅关闭时等待在途请求完成的上限。
	HTTPShutdownTimeout = 15 * time.Second
	// HTTPMaxHeaderBytes 用于限制 HTTP 请求头大小。
	HTTPMaxHeaderBytes = 1 << 20
)

// WithTimeout 基于父上下文创建带超时的上下文；父上下文为空时回退到 Background。
func WithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = RequestDefaultTimeout
	}

	if parent == nil {
		parent = context.TODO()
	}

	return context.WithTimeout(parent, timeout)
}

// WithDetachedTimeout 创建与父取消信号脱钩、但仍有独立超时的上下文，适合后台最佳努力任务。
func WithDetachedTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = AsyncDefaultTimeout
	}

	if parent == nil {
		// detached 任务没有父上下文时使用独立后台 root 超时。
		return context.WithTimeout(context.Background(), timeout)
	}

	return context.WithTimeout(context.WithoutCancel(parent), timeout)
}
