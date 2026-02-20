package ark

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	aiapp "f_ccg_video_back_ddd/internal/application/ai"
)

// AssetProxy 上游资源流代理适配器。
//
// scheme 与域名白名单校验由应用服务在调用前完成（见 application/ai.OpenAssetStream）。
type AssetProxy struct{}

var _ aiapp.AssetFetcher = (*AssetProxy)(nil)

// NewAssetProxy 构造资源代理适配器；运行配置由每次调用显式传入。
func NewAssetProxy() *AssetProxy {
	return &AssetProxy{}
}

// Open 拉取上游资源流；非 2xx/3xx 时读干错误体并转成 APIError。
func (p *AssetProxy) Open(
	ctx context.Context,
	runtime aiapp.RuntimeConfig,
	assetURL string,
	incomingHeaders http.Header,
) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构建资源代理请求失败: %w", err)
	}

	req.Header.Set("Accept", "*/*")

	// 透传给上游的条件/范围请求头（支持断点与缓存协商）。
	for _, key := range [...]string{"Range", "If-Range", "If-Modified-Since", "If-None-Match"} {
		if value := strings.TrimSpace(incomingHeaders.Get(key)); value != "" {
			req.Header.Set(key, value)
		}
	}

	resp, err := httpClientFor(runtime, false).Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 AI 资源失败: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, errorBodyLimit))
		_ = resp.Body.Close()

		if readErr != nil {
			return nil, fmt.Errorf("读取 AI 资源错误响应失败: %w", readErr)
		}

		return nil, parseAPIError(resp.StatusCode, body)
	}

	return resp, nil
}
