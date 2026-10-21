package video

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// DASHVariant 是 MPD 中的一个 Representation（DASH 资源下的视频或音频轨）。
type DASHVariant struct {
	ID          string `json:"id"`
	ContentType string `json:"contentType"` // video | audio | unknown
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	// Bandwidth 是 MPD 中的原始 bps。
	Bandwidth int `json:"bandwidth,omitempty"`
	// Bitrate 是 kbps，与 MP4 资源的展示单位保持一致。
	Bitrate  int    `json:"bitrate,omitempty"`
	Codecs   string `json:"codecs,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	BaseURL  string `json:"baseUrl,omitempty"`
	FileName string `json:"fileName,omitempty"`
	// FileSize 由最后一个 SegmentURL 的 mediaRange 末字节推算（单文件 DASH）。
	FileSize   int64  `json:"fileSize,omitempty"`
	Resolution string `json:"resolution,omitempty"`
	FrameRate  string `json:"frameRate,omitempty"`
}

// mpdXML 是用于 Representation 发现的最小 MPD 结构。
type mpdXML struct {
	XMLName xml.Name    `xml:"MPD"`
	Periods []mpdPeriod `xml:"Period"`
}

type mpdPeriod struct {
	AdaptationSets []mpdAdaptationSet `xml:"AdaptationSet"`
}

type mpdAdaptationSet struct {
	ContentType string              `xml:"contentType,attr"`
	MimeType    string              `xml:"mimeType,attr"`
	Codecs      string              `xml:"codecs,attr"`
	FrameRate   string              `xml:"frameRate,attr"`
	MaxWidth    int                 `xml:"maxWidth,attr"`
	MaxHeight   int                 `xml:"maxHeight,attr"`
	Reps        []mpdRepresentation `xml:"Representation"`
}

type mpdRepresentation struct {
	ID          string          `xml:"id,attr"`
	Bandwidth   int             `xml:"bandwidth,attr"`
	Width       int             `xml:"width,attr"`
	Height      int             `xml:"height,attr"`
	Codecs      string          `xml:"codecs,attr"`
	MimeType    string          `xml:"mimeType,attr"`
	FrameRate   string          `xml:"frameRate,attr"`
	BaseURL     string          `xml:"BaseURL"`
	SegmentList *mpdSegmentList `xml:"SegmentList"`
	SegmentBase *mpdSegmentBase `xml:"SegmentBase"`
}

type mpdSegmentList struct {
	Initialization *mpdRange   `xml:"Initialization"`
	SegmentURLs    []mpdSegURL `xml:"SegmentURL"`
}

type mpdSegmentBase struct {
	IndexRange string    `xml:"indexRange,attr"`
	Init       *mpdRange `xml:"Initialization"`
}

type mpdRange struct {
	Range string `xml:"range,attr"`
}

type mpdSegURL struct {
	MediaRange string `xml:"mediaRange,attr"`
	IndexRange string `xml:"indexRange,attr"`
}

var mediaRangeEndRe = regexp.MustCompile(`(?i)^(\d+)-(\d+)$`)

// xmlnsStripRe 去掉 xmlns 声明，使 encoding/xml 能在 MPEG-DASH 默认命名空间下
// 按本地元素名匹配，而不必给每个字段都打上 URN。
var xmlnsStripRe = regexp.MustCompile(`\sxmlns(?::[A-Za-z0-9_]+)?="[^"]*"`)

// IsDASHResource 判断资源是否为 DASH（format=dash 或 URL 以 .mpd 结尾/含 .mpd?）。
func IsDASHResource(r ResourceBrief) bool {
	if strings.EqualFold(strings.TrimSpace(r.Format), "dash") {
		return true
	}

	return strings.HasSuffix(strings.ToLower(r.FileURL), ".mpd") ||
		strings.Contains(strings.ToLower(r.FileURL), ".mpd?")
}

// MaxVariantBitrateKbps 返回指定 contentType 下最大的 kbps 码率；contentType 为空表示不过滤。
func MaxVariantBitrateKbps(variants []DASHVariant, contentType string) int {
	maxBitrate := 0
	for _, v := range variants {
		if contentType != "" && v.ContentType != contentType {
			continue
		}

		if v.Bitrate > maxBitrate {
			maxBitrate = v.Bitrate
		}
	}

	return maxBitrate
}

// ParseMPDVariants 从 MPD XML 字节中提取 Representation 列表。
//
// DASH MPD 几乎总会声明 xmlns="urn:mpeg:dash:schema:mpd:2011"；
// Go 的 encoding/xml 要求标签命名空间精确匹配，因此先剥离 xmlns 再按本地名匹配。
func ParseMPDVariants(data []byte) ([]DASHVariant, error) {
	cleaned := xmlnsStripRe.ReplaceAll(data, nil)

	var doc mpdXML
	if err := xml.Unmarshal(cleaned, &doc); err != nil {
		return nil, fmt.Errorf("xml unmarshal: %w", err)
	}

	out := make([]DASHVariant, 0, 8)

	for _, period := range doc.Periods {
		for _, as := range period.AdaptationSets {
			asType := classifyAdaptationContentType(as)
			for _, rep := range as.Reps {
				v := DASHVariant{
					ID:          strings.TrimSpace(rep.ID),
					ContentType: asType,
					Width:       rep.Width,
					Height:      rep.Height,
					Bandwidth:   rep.Bandwidth,
					Codecs:      firstNonEmpty(rep.Codecs, as.Codecs),
					MimeType:    firstNonEmpty(rep.MimeType, as.MimeType),
					BaseURL:     strings.TrimSpace(rep.BaseURL),
					FrameRate:   firstNonEmpty(rep.FrameRate, as.FrameRate),
				}
				if v.Bandwidth > 0 {
					v.Bitrate = (v.Bandwidth + 500) / 1000 // bps → kbps（四舍五入）
				}

				if v.BaseURL != "" {
					v.FileName = baseNameOf(strings.Split(v.BaseURL, "?")[0])
				}

				// AdaptationSet 未声明时，用 mime/codecs 推断内容类型。
				if v.ContentType == "unknown" {
					v.ContentType = classifyRepContentType(v)
				}

				switch v.ContentType {
				case "video":
					v.Resolution = ResolutionLabelFromSize(v.Width, v.Height)
				case "audio":
					v.Resolution = "音频"
				default:
					v.Resolution = firstNonEmpty(v.MimeType, "流")
				}

				v.FileSize = estimateRepFileSize(rep)
				out = append(out, v)
			}
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no representation in mpd")
	}

	sort.SliceStable(out, func(i, j int) bool {
		// 视频优先，其次按高度降序，再按带宽降序
		if out[i].ContentType != out[j].ContentType {
			return contentTypeRank(out[i].ContentType) < contentTypeRank(out[j].ContentType)
		}

		if out[i].Height != out[j].Height {
			return out[i].Height > out[j].Height
		}

		return out[i].Bandwidth > out[j].Bandwidth
	})

	return out, nil
}

func contentTypeRank(t string) int {
	switch t {
	case "video":
		return 0
	case "audio":
		return 1
	default:
		return 2
	}
}

func classifyAdaptationContentType(as mpdAdaptationSet) string {
	ct := strings.ToLower(strings.TrimSpace(as.ContentType))
	if ct == "video" || ct == "audio" {
		return ct
	}

	mime := strings.ToLower(as.MimeType)

	switch {
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	}

	if as.MaxHeight > 0 || as.MaxWidth > 0 {
		return "video"
	}

	return "unknown"
}

func classifyRepContentType(v DASHVariant) string {
	mime := strings.ToLower(v.MimeType)

	switch {
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	}

	codecs := strings.ToLower(v.Codecs)
	if strings.Contains(codecs, "avc") || strings.Contains(codecs, "hev") ||
		strings.Contains(codecs, "hvc") || strings.Contains(codecs, "vp9") ||
		strings.Contains(codecs, "av01") {
		return "video"
	}

	if strings.Contains(codecs, "mp4a") || strings.Contains(codecs, "opus") ||
		strings.Contains(codecs, "ac-3") || strings.Contains(codecs, "ec-3") {
		return "audio"
	}

	if v.Width > 0 || v.Height > 0 {
		return "video"
	}

	return "unknown"
}

// ResolutionLabelFromSize 尽可能把 WxH 映射为项目内的分辨率标签。
//
// 宽 >= 3840 的超宽片与 MapResolutionEnum 一样归入 4K。只看高度时，
// 3840×1632 会因为 1632 < 2000 被标成 1080p。
func ResolutionLabelFromSize(width, height int) string {
	if width >= 3840 || height >= 2160 {
		return string(enum.Resolution2160P)
	}

	h := height
	if h <= 0 && width > 0 {
		// 竖屏兜底
		h = width
	}

	switch {
	case h >= 2000:
		return string(enum.Resolution2160P)
	case h >= 1000:
		return string(enum.Resolution1080P)
	case h >= 700:
		return string(enum.Resolution720P)
	case h >= 300:
		return string(enum.Resolution360P)
	case h > 0:
		return fmt.Sprintf("%dp", h)
	default:
		return "自适应档位"
	}
}

func estimateRepFileSize(rep mpdRepresentation) int64 {
	maxEnd := int64(0)

	if rep.SegmentList != nil {
		for _, seg := range rep.SegmentList.SegmentURLs {
			if end := parseRangeEnd(seg.MediaRange); end > maxEnd {
				maxEnd = end
			}
		}

		if rep.SegmentList.Initialization != nil {
			if end := parseRangeEnd(rep.SegmentList.Initialization.Range); end > maxEnd {
				maxEnd = end
			}
		}
	}

	if rep.SegmentBase != nil {
		if end := parseRangeEnd(rep.SegmentBase.IndexRange); end > maxEnd {
			maxEnd = end
		}

		if rep.SegmentBase.Init != nil {
			if end := parseRangeEnd(rep.SegmentBase.Init.Range); end > maxEnd {
				maxEnd = end
			}
		}
	}

	if maxEnd <= 0 {
		return 0
	}

	// mediaRange 的末字节下标是闭区间 → size = end+1
	return maxEnd + 1
}

func parseRangeEnd(rangeAttr string) int64 {
	rangeAttr = strings.TrimSpace(rangeAttr)

	m := mediaRangeEndRe.FindStringSubmatch(rangeAttr)
	if len(m) != 3 {
		return 0
	}

	end, err := strconv.ParseInt(m[2], 10, 64)
	if err != nil {
		return 0
	}

	return end
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}

	return ""
}

// baseNameOf 取路径末段（等价 path.Base 对普通相对路径的行为）。
func baseNameOf(p string) string {
	if p == "" {
		return "."
	}

	trimmed := strings.TrimRight(p, "/")
	if trimmed == "" {
		return "/"
	}

	if i := strings.LastIndexByte(trimmed, '/'); i >= 0 {
		return trimmed[i+1:]
	}

	return trimmed
}

// DashDirPrefix 从 DASH manifest 的 file_url 中提取目录前缀。
//
// 例 "http://host/videos/8/dash/manifest.mpd" → "videos/8/dash/"。
func DashDirPrefix(fileURL string) string {
	trimmed := strings.TrimSpace(fileURL)
	if trimmed == "" {
		return ""
	}
	// 如果是完整 URL，取 Path 部分
	if u, err := parseAbsoluteURL(trimmed); err == nil && u != "" {
		trimmed = u
	}
	// 去掉查询参数（如有）
	if idx := strings.Index(trimmed, "?"); idx >= 0 {
		trimmed = trimmed[:idx]
	}

	trimmed = strings.TrimPrefix(trimmed, "/")
	// 找到 manifest.mpd 所在目录
	if idx := strings.LastIndex(trimmed, "/"); idx >= 0 {
		return trimmed[:idx+1] // 保留末尾 /
	}

	return ""
}
