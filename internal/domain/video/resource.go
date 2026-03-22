package video

import (
	"sort"
	"strings"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// ResourceBrief 视频资源摘要，供读模型与领域共用。
//
// JSON 同时用于接口响应与详情 meta 的 Redis 缓存，字段属于外部契约。
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

// ResourcePolicy 选择播放资源：去重、排序和 DASH 档位。
type ResourcePolicy struct{}

// Dedupe 按分辨率去重，保留「更优」的一条。
//
// 同分辨率下源片优先，其次文件更大、码率更高、ID 更大。
func (ResourcePolicy) Dedupe(list []ResourceBrief) []ResourceBrief {
	if len(list) <= 1 {
		return list
	}

	keep := make(map[enum.VideoResolution]ResourceBrief, len(list))
	for _, item := range list {
		prev, ok := keep[item.Resolution]
		if !ok || (ResourcePolicy{}).better(item, prev) {
			keep[item.Resolution] = item
		}
	}

	out := make([]ResourceBrief, 0, len(keep))
	for _, item := range keep {
		out = append(out, item)
	}

	return out
}

func (ResourcePolicy) better(a, b ResourceBrief) bool {
	if a.IsSource != b.IsSource {
		return a.IsSource
	}

	if a.FileSize != b.FileSize {
		return a.FileSize > b.FileSize
	}

	if a.Bitrate != b.Bitrate {
		return a.Bitrate > b.Bitrate
	}

	return a.ID > b.ID
}

// Sort 按清晰度从高到低稳定排序；同清晰度下源视频优先。
func (ResourcePolicy) Sort(items []ResourceBrief) {
	if len(items) == 0 {
		return
	}

	sort.SliceStable(items, func(i, j int) bool {
		ri := (ResourcePolicy{}).Rank(items[i].Resolution)

		rj := (ResourcePolicy{}).Rank(items[j].Resolution)
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

// Rank 返回分辨率的排序权重（数值越大越靠前）。
func (ResourcePolicy) Rank(res enum.VideoResolution) int {
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

// InitialPublishResolution 投稿时初始资源的默认分辨率。
func (ResourcePolicy) InitialResolution() enum.VideoResolution {
	return enum.Resolution360P
}

// InitialPublishFormat 依据文件名/路径推断初始资源格式，兜底 mp4。
func (ResourcePolicy) InitialFormat(fileName, filePath string) string {
	for _, candidate := range []string{strings.TrimSpace(fileName), strings.TrimSpace(filePath)} {
		if candidate == "" {
			continue
		}

		ext := strings.TrimPrefix(strings.ToLower((ResourcePolicy{}).fileExt(candidate)), ".")
		if ext != "" {
			return ext
		}
	}

	return "mp4"
}

// fileExt 取路径扩展名（等价 path.Ext，避免领域层引入 path 包的语义分歧）。
func (ResourcePolicy) fileExt(p string) string {
	for i := len(p) - 1; i >= 0 && p[i] != '/'; i-- {
		if p[i] == '.' {
			return p[i:]
		}
	}
	return ""
}
