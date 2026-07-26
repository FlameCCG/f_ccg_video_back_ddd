package video

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	"go.uber.org/zap"
)

const (
	// dashMPDFetchTimeout 拉取 MPD 的超时（与原 fetchDASHVariants 一致）。
	dashMPDFetchTimeout = 8 * time.Second
	// dashMPDMaxBytes MPD 读取上限：含大 SegmentList 的清单可能很大，8MB 足够。
	dashMPDMaxBytes = 8 << 20
)

// DASHVariantResolver 解析 DASH 资源的 MPD 清单，展开 Representation 列表。
type DASHVariantResolver interface {
	// Variants 拉取并解析 MPD；失败时返回错误（调用方按最佳努力忽略）。
	Variants(ctx context.Context, resourceID uint, mpdURL string) ([]domain.DASHVariant, error)
}

// HTTPDASHVariantResolver 通过 HTTP 拉取 MPD 的解析器。
//
// 原实现在每次调用时新建 http.Client；这里改为构造期持有一个客户端实例，
// 超时值保持一致，避免包级单例同时减少连接建立开销。
type HTTPDASHVariantResolver struct {
	client *http.Client
}

var _ DASHVariantResolver = (*HTTPDASHVariantResolver)(nil)

// NewHTTPDASHVariantResolver 构造基于 HTTP 的 MPD 解析器。
func NewHTTPDASHVariantResolver() *HTTPDASHVariantResolver {
	return &HTTPDASHVariantResolver{client: &http.Client{Timeout: dashMPDFetchTimeout}}
}

// Variants 拉取并解析 MPD。
func (r *HTTPDASHVariantResolver) Variants(
	ctx context.Context,
	resourceID uint,
	mpdURL string,
) ([]domain.DASHVariant, error) {
	variants, err := r.fetch(ctx, mpdURL)
	if err != nil {
		zap.L().Warn("parse DASH mpd variants failed",
			zap.Uint("resourceId", resourceID),
			zap.String("url", mpdURL),
			zap.Error(err),
		)

		return nil, err
	}

	return variants, nil
}

func (r *HTTPDASHVariantResolver) fetch(ctx context.Context, mpdURL string) ([]domain.DASHVariant, error) {
	mpdURL = trimSpace(mpdURL)
	if mpdURL == "" {
		return nil, errEmptyMPDURL
	}

	// 优先用原始 URL 拉取（response-content-type 查询参数对 HTTP GET 无影响）。
	reqCtx, cancel := context.WithTimeout(orTODO(ctx), dashMPDFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, mpdURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/dash+xml,application/xml,text/xml,*/*")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("mpd http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, dashMPDMaxBytes))
	if err != nil {
		return nil, err
	}

	return domain.ParseMPDVariants(body)
}

var errEmptyMPDURL = errors.New("empty mpd url")
