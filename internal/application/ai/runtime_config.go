package ai

import (
	"context"

	"f_ccg_video_back_ddd/pkg/ctxutil"
)

// RuntimeConfig 是一次 AI 公开用例调用捕获的上游运行配置。
//
// 它属于应用层端口契约，不暴露配置机制细节。值在用例入口完成
// 规范化后按值传递，因此配置热更新不会让同一次调用混用不同版本的地址、密钥或超时。
type RuntimeConfig struct {
	ChatModelBaseURL string
	ChatModelAPIKey  string
	DoubaoAPIKey     string
	TimeoutSec       int
}

// WithContext 给一次完整用例设置预算；已有更短的父截止时间仍然有效。
func (c RuntimeConfig) WithContext(parent context.Context) (context.Context, context.CancelFunc) {
	return ctxutil.WithTimeout(parent, ctxutil.AITimeout(c.TimeoutSec))
}
