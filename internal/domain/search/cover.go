package search

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
)

// MaxCoverImageBytes 封面图进入多模态向量前的体积上限，超过则退化为纯文本向量.
const MaxCoverImageBytes = 8 << 20

// ErrCoverUnavailable 表示封面图无法解析为方舟可消费的形式（缺失、过大、非图片等）.
var ErrCoverUnavailable = errors.New("cover image unavailable for embedding")

// IsDataURI 判断封面是否已经是 data URI（可直接交给多模态接口）.
func IsDataURI(cover string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(cover)), "data:")
}

// JoinAssetBaseURL 用配置的资源前缀把相对路径补成绝对地址；前缀为空时返回空串.
func JoinAssetBaseURL(baseURL, relative string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return ""
	}

	return baseURL + "/" + strings.TrimLeft(strings.ReplaceAll(strings.TrimSpace(relative), "\\", "/"), "/")
}

// IsPubliclyReachableHost 判断方舟侧能否直接拉取该地址。
//
// 判不准时一律按「拉不到」处理，让本机抓取兜底——多花一点带宽，但不会静默丢掉图像模态。
func IsPubliclyReachableHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}

	// 无点主机名（如 minio、localhost）只能在集群内解析
	if !strings.Contains(host, ".") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return false
	}

	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified()
	}

	return true
}

// SafeJoinUnderRoot 拼接路径并阻止穿越到存储根目录之外。
//
// 封面路径本身已包含存储根前缀（如 uploads/xxx.jpg），因此需要先去重再拼。
func SafeJoinUnderRoot(root, relative string) (string, error) {
	cleanedRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", fmt.Errorf("%w: resolve storage root failed: %w", ErrCoverUnavailable, err)
	}

	candidate := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(strings.TrimSpace(relative), "\\", "/")))
	if !filepath.IsAbs(candidate) {
		rootBase := filepath.Base(cleanedRoot)
		// cover 形如 uploads/... 时，直接与 uploads 根目录拼会变成 uploads/uploads/...
		if first, _, found := strings.Cut(filepath.ToSlash(candidate), "/"); found && first == rootBase {
			candidate = filepath.Join(cleanedRoot, strings.TrimPrefix(filepath.ToSlash(candidate), first+"/"))
		} else {
			candidate = filepath.Join(cleanedRoot, candidate)
		}
	}

	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("%w: resolve cover path failed: %w", ErrCoverUnavailable, err)
	}

	relPath, err := filepath.Rel(cleanedRoot, absolute)
	if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: cover path escapes the storage root", ErrCoverUnavailable)
	}

	return absolute, nil
}
