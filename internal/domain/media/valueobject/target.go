package valueobject

import (
	"slices"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// 转码管线名称（进度上报与断点目录命名空间共用）。
const (
	// TranscodePipelineMP4 多分辨率 MP4 管线。
	TranscodePipelineMP4 = "mp4"
	// TranscodePipelineDASH DASH 自适应流管线。
	TranscodePipelineDASH = "dash"
	// TranscodePipelineSkipped 标记已配置但不会运行的管线。
	TranscodePipelineSkipped = "skipped"
)

// DashManifestName DASH manifest 的固定文件名。
const DashManifestName = "manifest.mpd"

// TranscodeTarget 描述一路转码输出目标（一个分辨率 / 一个 representation）。
type TranscodeTarget struct {
	// ID 目标唯一标识（如 mp4:1080:high、dash:2160:high）。
	ID string
	// Format 所属管线（mp4 / dash）。
	Format string
	// Role 目标角色：source（源片规范化）或 derived（派生阶梯）。
	Role string
	// Resolution 目标像素高。
	Resolution uint16
	// Quality 画质档位：0=按分辨率, 1=高码率档, 2=中码率档。
	Quality QualityMode
	// DBResolution 落库用的分辨率枚举。
	DBResolution enum.VideoResolution
	// IsVIP 是否为大会员资源。
	IsVIP bool
	// IsSource 是否为源片规范化目标。
	IsSource bool
	// KeepSource 为 true 时使用 null 滤镜，不改变像素尺寸（禁止上采样）。
	KeepSource bool
	// MaxrateUseSource 为 true 时 -maxrate 直接按源码率预算（非高码率源片/同源高规范化，不做 reduce）。
	MaxrateUseSource bool
	// FileName 产物文件名。
	FileName string
	// RelativePath 产物在存储中的相对路径。
	RelativePath string
	// Label 上传进度日志标签。
	Label string
	// MapLabel filter_complex 输出标签（由 ffmpeg 参数构造阶段回填）。
	MapLabel string
}

// BuildDashResolutions 计算 DASH 阶梯：非标准源高度向下归入站点支持的标准档位。
// classHeight 与 MP4 的宽高分类一致；按宽度归为 4K 的超宽源保留 2160 档位，
// 实际像素由 KeepDASHSourcePixels 保持不变，绝不把源高上采样到 2160。
func (p *TranscodePolicy) BuildDashResolutions(
	targetRes []uint16,
	pixelHeight uint16,
	classHeight uint16,
) []uint16 {
	set := make(map[uint16]struct{})
	topHeight := DASHTopResolutionHeight(pixelHeight, classHeight)

	addAtOrBelowTop := func(r uint16) {
		if r == 0 {
			return
		}

		if topHeight > 0 && r > topHeight {
			return
		}

		set[r] = struct{}{}
	}

	if p != nil && len(p.TranscodeResolutions) > 0 {
		for _, r := range p.TranscodeResolutions {
			addAtOrBelowTop(r)
		}
	} else {
		for _, r := range targetRes {
			addAtOrBelowTop(r)
		}
	}

	if topHeight > 0 {
		set[topHeight] = struct{}{}
	}

	out := make([]uint16, 0, len(set))
	for r := range set {
		out = append(out, r)
	}

	slices.Sort(out)

	return out
}

// DASHTopResolutionHeight 返回源对应的最高标准 DASH 档位。
// 低于最低 360p 档的源保持原高；按宽度归为 4K 的源沿用 MP4 的 2160 分类。
func DASHTopResolutionHeight(pixelHeight, classHeight uint16) uint16 {
	var topHeight uint16

	switch {
	case pixelHeight == 0:
		return 0
	case pixelHeight < 360:
		topHeight = pixelHeight
	case pixelHeight < 720:
		topHeight = 360
	case pixelHeight < 1080:
		topHeight = 720
	case pixelHeight < 2160:
		topHeight = 1080
	default:
		topHeight = 2160
	}

	if classHeight == 2160 && topHeight < classHeight {
		return classHeight
	}

	return topHeight
}

// KeepDASHSourcePixels 判断某档位是否代表源片原生像素。
// 3840×1632 等超宽 4K 使用 2160 作为档位标签，但滤镜保持实际 3840×1632。
func KeepDASHSourcePixels(resolution, pixelHeight, classHeight uint16) bool {
	if pixelHeight == 0 {
		return false
	}

	if resolution == pixelHeight {
		return true
	}

	return classHeight == 2160 &&
		resolution == classHeight &&
		pixelHeight < classHeight
}
