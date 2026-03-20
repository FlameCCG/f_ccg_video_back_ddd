package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	aierrors "f_ccg_video_back_ddd/internal/domain/ai/domainerr"
	aivo "f_ccg_video_back_ddd/internal/domain/ai/valueobject"
)

// OpenAssetStream 代理拉取上游资源流（仅放行白名单域名，防 SSRF）.
func (s *Service) OpenAssetStream(
	ctx context.Context,
	assetURL string,
	incomingHeaders http.Header,
) (*http.Response, error) {
	snapshot, err := s.snapshot()
	if err != nil {
		return nil, err
	}

	parsed, err := url.Parse(strings.TrimSpace(assetURL))
	if err != nil {
		return nil, fmt.Errorf("解析资源地址失败: %w", err)
	}

	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, aierrors.ErrUnsupportedAssetScheme
	}

	if !aivo.IsAllowedAssetHost(parsed.Hostname()) {
		return nil, aierrors.ErrAssetHostNotAllowed
	}

	return s.assets.Open(ctx, snapshot.runtime, parsed.String(), incomingHeaders)
}
