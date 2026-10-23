// Package ark 实现AI 模型与媒体生成上游的外部协议适配。
//
// chat_model.go/image_model.go/video_task.go 分别处理聊天、图片和视频能力，asset.go 代理资源拉取。
// 应用入口捕获运行配置后按值传入；本包负责 HTTP/SDK、流读取和协议转换，不决定业务资格。
// 流、响应体和上下文都带有资源生命周期，调用方与适配器需要按端口约定关闭和传播取消。
//
// 上层依赖端口而非客户端库，测试可替换端口或 HTTP 服务以验证协议边界。
package ark

import (
	"strings"
	"time"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"
)

// arkTimeout 把配置的超时秒数转成方舟 SDK 所需的可选超时。
func arkTimeout(runtime aiapp.RuntimeConfig) *time.Duration {
	if runtime.TimeoutSec <= 0 {
		return nil
	}

	timeout := time.Duration(runtime.TimeoutSec) * time.Second

	return &timeout
}

// doubaoAPIKey 返回去空格后的豆包模型 API Key。
func doubaoAPIKey(runtime aiapp.RuntimeConfig) string {
	return strings.TrimSpace(runtime.DoubaoAPIKey)
}
