package valueobject

import (
	"fmt"
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
	FileSize   int64    `json:"fileSize,omitempty"`
	Resolution string   `json:"resolution,omitempty"`
	FrameRate  string   `json:"frameRate,omitempty"`
	Files      []string `json:"-"`
}

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
