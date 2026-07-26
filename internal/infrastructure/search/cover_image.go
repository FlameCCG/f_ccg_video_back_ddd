package search

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/search"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
)

// coverFetchTimeout 抓取封面图的超时保护，由调用方 ctx 再收紧一次.
const coverFetchTimeout = 10 * time.Second

// CoverResolver 把视频封面解析成方舟多模态接口可直接消费的地址。
//
// 原实现用包级 http.Client 与 global.Config()；此处改为结构体字段 + 注入配置。
type CoverResolver struct {
	cfg *config.Provider
	// httpClient 抓取内网/本机封面图时复用，避免每次索引都新建连接池。
	httpClient *http.Client
}

// NewCoverResolver 构造封面图解析器。
func NewCoverResolver(cfgProvider *config.Provider) *CoverResolver {
	return &CoverResolver{
		cfg:        cfgProvider,
		httpClient: &http.Client{Timeout: coverFetchTimeout},
	}
}

// Resolve 解析封面地址。
//
// 优先级：
//  1. 已经是 data URI —— 原样返回；
//  2. 公网可达的 http(s) 地址 —— 直接交给方舟拉取，省一次本机流量；
//  3. 内网 / 本机 http(s) 地址 —— 方舟拉不到，改为本机抓取后转成 data URI；
//  4. 相对路径 —— 先按 assetBaseURL 补全，补不出来就按本地存储从磁盘读。
//
// 无法解析时返回包装了 domain.ErrCoverUnavailable 的错误，调用方据此退化为纯文本向量。
func (r *CoverResolver) Resolve(ctx context.Context, cover, assetBaseURL string) (string, error) {
	cover = strings.TrimSpace(cover)
	if cover == "" {
		return "", domain.ErrCoverUnavailable
	}

	if domain.IsDataURI(cover) {
		return cover, nil
	}

	if parsed, err := url.Parse(cover); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		if domain.IsPubliclyReachableHost(parsed.Hostname()) {
			return cover, nil
		}

		return r.fetchImageAsDataURI(ctx, cover)
	}

	if absolute := domain.JoinAssetBaseURL(assetBaseURL, cover); absolute != "" {
		if parsed, err := url.Parse(absolute); err == nil && domain.IsPubliclyReachableHost(parsed.Hostname()) {
			return absolute, nil
		}

		return r.fetchImageAsDataURI(ctx, absolute)
	}

	return r.readLocalImageAsDataURI(cover)
}

// fetchImageAsDataURI 抓取图片并转成 base64 数据 URI。
func (r *CoverResolver) fetchImageAsDataURI(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("%w: build cover request failed: %w", domain.ErrCoverUnavailable, err)
	}

	req.Header.Set("Accept", "image/*")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: fetch cover failed: %w", domain.ErrCoverUnavailable, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%w: fetch cover got status %d", domain.ErrCoverUnavailable, resp.StatusCode)
	}

	// 多读 1 字节用于判断是否超限，避免把超大对象整个读进内存
	data, err := io.ReadAll(io.LimitReader(resp.Body, domain.MaxCoverImageBytes+1))
	if err != nil {
		return "", fmt.Errorf("%w: read cover failed: %w", domain.ErrCoverUnavailable, err)
	}

	return buildImageDataURI(data, resp.Header.Get("Content-Type"))
}

// readLocalImageAsDataURI 按本地存储根目录读取封面并转成数据 URI。
func (r *CoverResolver) readLocalImageAsDataURI(cover string) (string, error) {
	root := r.localStorageRoot()
	if root == "" {
		return "", fmt.Errorf(
			"%w: local storage is not enabled and assetBaseURL is not configured",
			domain.ErrCoverUnavailable,
		)
	}

	path, err := domain.SafeJoinUnderRoot(root, cover)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%w: stat cover failed: %w", domain.ErrCoverUnavailable, err)
	}

	if info.Size() > domain.MaxCoverImageBytes {
		return "", fmt.Errorf("%w: cover is %d bytes, over the %d limit",
			domain.ErrCoverUnavailable, info.Size(), domain.MaxCoverImageBytes)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%w: read cover failed: %w", domain.ErrCoverUnavailable, err)
	}

	return buildImageDataURI(data, "")
}

// localStorageRoot 返回本地存储根目录；未启用本地存储时返回空串。
func (r *CoverResolver) localStorageRoot() string {
	if r == nil || r.cfg == nil {
		return ""
	}

	cfg := r.cfg.Get()
	if cfg == nil || cfg.Site == nil || cfg.Site.Storage == nil || cfg.Site.Storage.Local == nil {
		return ""
	}

	if !cfg.Site.Storage.Local.Enable {
		return ""
	}

	return strings.TrimSpace(cfg.Site.Storage.Local.Path)
}

// buildImageDataURI 校验体积与类型后生成 base64 数据 URI。
func buildImageDataURI(data []byte, declaredContentType string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("%w: cover is empty", domain.ErrCoverUnavailable)
	}

	if len(data) > domain.MaxCoverImageBytes {
		return "", fmt.Errorf("%w: cover is over the %d bytes limit",
			domain.ErrCoverUnavailable, domain.MaxCoverImageBytes)
	}

	contentType := strings.TrimSpace(strings.Split(declaredContentType, ";")[0])
	if !strings.HasPrefix(contentType, "image/") {
		contentType = strings.Split(http.DetectContentType(data), ";")[0]
	}

	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("%w: cover is not an image (%s)", domain.ErrCoverUnavailable, contentType)
	}

	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}
