package ai

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/res"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// assetProxyRoute 本站 AI 资源代理路由（与 router 中注册的路径一致）。
const assetProxyRoute = "/v1/common/ai/assets"

func (h *Handler) failWithAIError(c *gin.Context, _ string, err error) {
	res.RecordFailure(apperr.Wrap("handler/ai/helpers", nil, err), c)
	res.FailWithMsg(userFacingAIError(err), c)
}

// userFacingAIError 将可展示的配置/参数类错误直接返回前端，其余用通用文案避免泄漏内部细节。
func userFacingAIError(err error) string {
	const fallback = "AI 服务暂时不可用，请稍后重试"
	if err == nil {
		return fallback
	}

	return clienterror.Message(err)
}

func copyUpstreamHeaders(dst, src http.Header) {
	for key, values := range src {
		if strings.EqualFold(key, "Content-Length") {
			continue
		}

		dst.Del(key)

		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// rewriteAIAssetURLs 把响应中的上游图床地址重写为本站代理地址。
func rewriteAIAssetURLs(payload any) any {
	switch value := payload.(type) {
	case map[string]any:
		rewritten := make(map[string]any, len(value))

		for key, item := range value {
			if rawURL, ok := item.(string); ok && isAIAssetURL(rawURL) && isAssetURLField(key) {
				rewritten[key] = buildAIAssetProxyURL(rawURL)

				continue
			}

			rewritten[key] = rewriteAIAssetURLs(item)
		}

		return rewritten
	case []any:
		rewritten := make([]any, len(value))
		for index, item := range value {
			rewritten[index] = rewriteAIAssetURLs(item)
		}

		return rewritten
	default:
		return payload
	}
}

func isAssetURLField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "url", "image_url", "imageurl", "video_url", "videourl":
		return true
	default:
		return false
	}
}

// isAIAssetURL 判断是否为需走本站代理的上游 AI 资源地址（火山等图床域名）。
func isAIAssetURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	for _, suffix := range []string{".volces.com", ".volcengine.com", ".byteimg.com", ".bytedance.com"} {
		if strings.HasSuffix(host, suffix) || strings.TrimPrefix(suffix, ".") == host {
			return true
		}
	}

	return false
}

func buildAIAssetProxyURL(raw string) string {
	params := url.Values{}
	params.Set("url", strings.TrimSpace(raw))

	return assetProxyRoute + "?" + params.Encode()
}
