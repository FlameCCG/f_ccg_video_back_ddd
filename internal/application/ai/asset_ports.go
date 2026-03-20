package ai

import (
	"context"
	"net/http"
)

// AssetFetcher 上游资源代理拉取端口。
//
// 调用方（应用服务）已完成 scheme 与域名白名单校验，适配器只做转发。
type AssetFetcher interface {
	// Open 拉取资源流；返回值的 Body 由调用方负责关闭。
	Open(
		ctx context.Context,
		runtime RuntimeConfig,
		assetURL string,
		incomingHeaders http.Header,
	) (*http.Response, error)
}
