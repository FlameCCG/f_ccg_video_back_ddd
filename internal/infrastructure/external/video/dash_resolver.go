// Package video 实现视频详情所需的远程 DASH MPD 读取的外部协议适配。
//
// dash_resolver.go 负责发请求和读取清单，再交给 domain/video 的纯解析规则。
// 网络超时、响应大小和 URL 解析属于适配器职责；返回的资源还要按详情读模型组织。
//
// 上层依赖端口而非客户端库，测试可替换端口或 HTTP 服务以验证协议边界。
package video

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/storage"

	"go.uber.org/zap"
)

const (
	// dashMPDFetchTimeout 拉取 MPD 的超时（与原 fetchDASHVariants 一致）。
	dashMPDFetchTimeout = 8 * time.Second
	// dashMPDMaxBytes MPD 读取上限：含大 SegmentList 的清单可能很大，8MB 足够。
	dashMPDMaxBytes = 8 << 20
)

// HTTPDASHVariantResolver 通过 HTTP 拉取 MPD 的解析器。
//
// 原实现在每次调用时新建 http.Client；这里改为构造期持有一个客户端实例，
// 超时值保持一致，避免包级单例同时减少连接建立开销。
type HTTPDASHVariantResolver struct {
	client *http.Client
	logger *zap.Logger
	stored *storage.Resolver
}

var _ app.DASHVariantResolver = (*HTTPDASHVariantResolver)(nil)

// NewHTTPDASHVariantResolver 构造基于 HTTP 的 MPD 解析器。
func NewHTTPDASHVariantResolver(logger *zap.Logger) *HTTPDASHVariantResolver {
	return &HTTPDASHVariantResolver{
		client: &http.Client{Timeout: dashMPDFetchTimeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 || req.URL.Host != via[0].URL.Host || req.URL.Scheme != via[0].URL.Scheme {
				return errors.New("DASH redirect origin mismatch")
			}
			return nil
		}},
		logger: logger,
	}
}

// NewStoredDASHVariantResolver reads the configured storage directly. A single
// directory listing supplies all segment sizes, including for long local videos
// whose root-relative manifest URL cannot be fetched with an HTTP client.
func NewStoredDASHVariantResolver(stored *storage.Resolver, logger *zap.Logger) *HTTPDASHVariantResolver {
	r := NewHTTPDASHVariantResolver(logger)
	r.stored = stored
	return r
}

// Variants 拉取并解析 MPD。
func (r *HTTPDASHVariantResolver) Variants(
	ctx context.Context,
	resourceID uint,
	mpdURL string,
) ([]domain.DASHVariant, error) {
	variants, err := r.fetch(ctx, mpdURL)
	if err != nil {
		r.logger.Warn("parse DASH mpd variants failed",
			zap.Uint("resourceId", resourceID),
			zap.String("url", mpdURL),
			zap.Error(err),
		)

		return nil, err
	}

	return variants, nil
}

func (r *HTTPDASHVariantResolver) fetch(ctx context.Context, mpdURL string) ([]domain.DASHVariant, error) {
	mpdURL = strings.TrimSpace(mpdURL)
	if mpdURL == "" {
		return nil, errEmptyMPDURL
	}

	// 优先用原始 URL 拉取（response-content-type 查询参数对 HTTP GET 无影响）。
	if ctx == nil {
		ctx = context.TODO()
	}
	reqCtx, cancel := context.WithTimeout(ctx, dashMPDFetchTimeout)
	defer cancel()
	if r.stored != nil {
		body, sizes, err := r.stored.ReadDASHManifest(reqCtx, mpdURL, dashMPDMaxBytes)
		if err != nil {
			return nil, err
		}
		variants, err := domain.ParseMPDVariants(body)
		if err != nil {
			return nil, err
		}
		for i := range variants {
			for _, file := range variants[i].Files {
				name := path.Clean(path.Join(variants[i].BaseURL, file))
				size, ok := sizes[name]
				if !ok {
					return nil, fmt.Errorf("DASH segment size unavailable: %s", name)
				}
				variants[i].FileSize += size
			}
		}
		return variants, nil
	}

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

	body, err := io.ReadAll(io.LimitReader(resp.Body, dashMPDMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > dashMPDMaxBytes {
		return nil, errors.New("DASH manifest exceeds size limit")
	}

	variants, err := domain.ParseMPDVariants(body)
	if err != nil {
		return nil, err
	}
	base, err := url.Parse(mpdURL)
	if err != nil {
		return nil, err
	}
	for i := range variants {
		if len(variants[i].Files) == 0 {
			continue
		}
		var total int64
		var firstErr error
		var mu sync.Mutex
		var wg sync.WaitGroup
		jobs := make(chan string)
		for worker := 0; worker < 8; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for name := range jobs {
					ref, err := url.Parse(name)
					if err == nil {
						objectURL := base.ResolveReference(ref)
						// Segments are generated beside the manifest; never probe arbitrary hosts.
						if objectURL.Host != base.Host || objectURL.Scheme != base.Scheme {
							err = errors.New("DASH segment origin mismatch")
						} else {
							var head *http.Request
							head, err = http.NewRequestWithContext(reqCtx, http.MethodHead, objectURL.String(), nil)
							if err == nil {
								var response *http.Response
								response, err = r.client.Do(head)
								if err == nil {
									_ = response.Body.Close()
									if response.StatusCode < 200 || response.StatusCode >= 300 || response.ContentLength < 0 {
										err = errors.New("DASH segment size unavailable")
									} else {
										mu.Lock()
										total += response.ContentLength
										mu.Unlock()
									}
								}
							}
						}
					}
					if err != nil {
						mu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						mu.Unlock()
					}
				}
			}()
		}
		for _, name := range variants[i].Files {
			ref := name
			if variants[i].BaseURL != "" {
				ref = variants[i].BaseURL + name
			}
			select {
			case jobs <- ref:
			case <-reqCtx.Done():
			}
		}
		close(jobs)
		wg.Wait()
		if firstErr != nil {
			return nil, firstErr
		}
		if err := reqCtx.Err(); err != nil {
			return nil, err
		}
		variants[i].FileSize = total
	}
	return variants, nil
}

var errEmptyMPDURL = errors.New("empty mpd url")
