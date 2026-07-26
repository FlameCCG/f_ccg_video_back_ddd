package video

import (
	"net/url"
	"sort"
	"strings"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// ResourceBrief 视频资源摘要（读模型与领域共用的值对象）。
//
// json tag 与原 service/video_service.VideoResourceBrief 逐字一致：
// 它既出现在接口响应里，也随详情 meta 一起写入 Redis 缓存，属于外部契约。
type ResourceBrief struct {
	ID             uint                 `json:"id"`
	Resolution     enum.VideoResolution `json:"resolution"`
	SourceFileName string               `json:"sourceFileName"`
	FileURL        string               `json:"fileUrl"`
	FileSize       int64                `json:"fileSize"`
	Bitrate        int                  `json:"bitrate"`
	Format         string               `json:"format"`
	Codec          string               `json:"codec"`
	IsVip          bool                 `json:"isVip"`
	IsSource       bool                 `json:"isSource"`
	// Variants 仅 DASH（format=dash / .mpd）资源填充，来自 MPD Representations。
	Variants []DASHVariant `json:"variants,omitempty"`
}

// DedupeResourceBriefs 按（分辨率, 是否源片）去重，保留「更优」的一条。
//
// 与原 dedupeResourceBriefs 完全一致：文件更大优先，其次码率更高，最后 ID 更大。
func DedupeResourceBriefs(list []ResourceBrief) []ResourceBrief {
	if len(list) <= 1 {
		return list
	}

	type key struct {
		res      enum.VideoResolution
		isSource bool
	}

	keep := make(map[key]ResourceBrief, len(list))
	for _, item := range list {
		k := key{res: item.Resolution, isSource: item.IsSource}

		prev, ok := keep[k]
		if !ok || isBetterResourceBrief(item, prev) {
			keep[k] = item
		}
	}

	out := make([]ResourceBrief, 0, len(keep))
	for _, item := range keep {
		out = append(out, item)
	}

	return out
}

func isBetterResourceBrief(a, b ResourceBrief) bool {
	if a.FileSize != b.FileSize {
		return a.FileSize > b.FileSize
	}

	if a.Bitrate != b.Bitrate {
		return a.Bitrate > b.Bitrate
	}

	return a.ID > b.ID
}

// SortResourceBriefs 按清晰度从高到低稳定排序；同清晰度下源视频优先。
func SortResourceBriefs(items []ResourceBrief) {
	if len(items) == 0 {
		return
	}

	sort.SliceStable(items, func(i, j int) bool {
		ri := ResolutionRank(items[i].Resolution)

		rj := ResolutionRank(items[j].Resolution)
		if ri != rj {
			// 清晰度按高到低排序
			return ri > rj
		}

		if items[i].IsSource != items[j].IsSource {
			// 同清晰度下，源视频优先（便于“发布后立即可看”的默认策略）
			return items[i].IsSource
		}

		return false
	})
}

// ResolutionRank 返回分辨率的排序权重（数值越大越靠前）。
func ResolutionRank(res enum.VideoResolution) int {
	switch res {
	case enum.ResolutionAuto:
		return 100000 // DASH 自适应流排在最前，作为默认项
	case enum.Resolution2160P:
		return 2160
	case enum.Resolution1080PHigh:
		return 1090
	case enum.Resolution1080P:
		return 1080
	case enum.Resolution720P:
		return 720
	case enum.Resolution360P:
		return 360
	default:
		return 0
	}
}

// EnsurePlayableContentTypeURL 给 MinIO 对象 URL 追加 response-content-type，
// 修正历史对象 octet-stream + nosniff 导致 Chrome 拒绝播放的问题。
//
// 与原 ensurePlayableContentTypeURL 逐行一致。
func EnsurePlayableContentTypeURL(fileURL string) string {
	raw := strings.TrimSpace(fileURL)
	if raw == "" {
		return raw
	}

	low := strings.ToLower(raw)
	if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
		return raw
	}

	if strings.Contains(low, "response-content-type=") {
		return NormalizeMediaURL(raw)
	}

	base := raw
	if idx := strings.IndexByte(low, '?'); idx >= 0 {
		base = raw[:idx]
	}

	ext := strings.ToLower(urlPathExt(base))

	var ct string

	switch ext {
	case ".mp4":
		ct = "video/mp4"
	case ".m3u8":
		ct = "application/x-mpegURL"
	case ".ts":
		ct = "video/mp2t"
	default:
		return NormalizeMediaURL(raw)
	}

	sep := "?"
	if strings.Contains(raw, "?") {
		sep = "&"
	}

	return NormalizeMediaURL(raw + sep + "response-content-type=" + url.QueryEscape(ct))
}

// urlPathExt 取 URL 路径的扩展名（不引入 path/filepath：URL 路径统一用 /）。
func urlPathExt(p string) string {
	if p == "" {
		return ""
	}

	if i := strings.LastIndexByte(p, '.'); i >= 0 {
		if j := strings.LastIndexByte(p, '/'); j >= 0 && j > i {
			return ""
		}

		return p[i:]
	}

	return ""
}

// parseAbsoluteURL 若 raw 是带 scheme+host 的绝对 URL，则返回其 Path，否则返回错误。
func parseAbsoluteURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	if u.Scheme == "" || u.Host == "" {
		return "", errNotAbsoluteURL
	}

	return u.Path, nil
}

// NormalizeMediaURL 规范化媒体 URL 的转义路径；解析失败时原样返回。
func NormalizeMediaURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return raw
	}

	escaped := (&url.URL{Path: u.Path}).EscapedPath()
	if escaped != "" {
		u.RawPath = escaped
	}

	return u.String()
}

// InitialPublishResolution 投稿时初始资源的默认分辨率。
func InitialPublishResolution() enum.VideoResolution {
	return enum.Resolution360P
}

// InitialPublishFormat 依据文件名/路径推断初始资源格式，兜底 mp4。
func InitialPublishFormat(fileName, filePath string) string {
	for _, candidate := range []string{strings.TrimSpace(fileName), strings.TrimSpace(filePath)} {
		if candidate == "" {
			continue
		}

		ext := strings.TrimPrefix(strings.ToLower(pathExtOf(candidate)), ".")
		if ext != "" {
			return ext
		}
	}

	return "mp4"
}

// pathExtOf 取路径扩展名（等价 path.Ext，避免领域层引入 path 包的语义分歧）。
func pathExtOf(p string) string {
	for i := len(p) - 1; i >= 0 && p[i] != '/'; i-- {
		if p[i] == '.' {
			return p[i:]
		}
	}

	return ""
}
