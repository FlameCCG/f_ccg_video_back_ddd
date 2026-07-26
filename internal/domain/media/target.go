package media

import (
	"fmt"
	"path"
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

// NewMP4Target 构造一路 MP4 输出目标。
func NewMP4Target(
	id string,
	resolution uint16,
	quality QualityMode,
	role, fileName string,
	dbResolution enum.VideoResolution,
	isVIP, isSource, keepSource bool,
	task TranscodeTask,
) TranscodeTarget {
	return TranscodeTarget{
		ID:           id,
		Format:       TranscodePipelineMP4,
		Role:         role,
		Resolution:   resolution,
		Quality:      quality,
		DBResolution: dbResolution,
		IsVIP:        isVIP,
		IsSource:     isSource,
		KeepSource:   keepSource,
		FileName:     fileName,
		RelativePath: path.Join(task.OutputPath, fileName),
		Label:        fmt.Sprintf("video-%d-%s", task.VideoID, fileName),
	}
}

// SelectResolutions 过滤出严格低于源高度的阶梯档（禁止上采样）。
func SelectResolutions(resList []uint16, maxHeight int) []uint16 {
	targets := make([]uint16, 0, len(resList))
	for _, r := range resList {
		if int(r) < maxHeight {
			targets = append(targets, r)
		}
	}

	return targets
}

// BuildSourceNormalizeTargets 产出源片规范化目标。
//
// 高码率：QualityModeHigh + High reduce。
// 非高码率：按档位高 2160→High / 1080→Medium / 其余→None；KeepSource 不改像素尺寸。
// maxrate：非高码率走 remux 预算（高效源编解码会抬到 H264 友好码率）。
func BuildSourceNormalizeTargets(
	task TranscodeTask,
	pixelHeight uint16,
	classHeight uint16,
	sourceResolution enum.VideoResolution,
	highBitrate bool,
) []TranscodeTarget {
	dbResolution := sourceResolution
	if highBitrate && dbResolution == enum.Resolution1080P {
		dbResolution = enum.Resolution1080PHigh
	}

	// 画质档位按分类高度（真 4K 档用 High），缩放高度用真实像素。
	qualityHeight := classHeight
	if qualityHeight == 0 {
		qualityHeight = pixelHeight
	}

	quality := QualityModeHigh
	if !highBitrate {
		quality = QualityForNonHighSourceHeight(qualityHeight)
	}

	scaleH := pixelHeight
	if scaleH == 0 {
		scaleH = classHeight
	}

	target := NewMP4Target(
		fmt.Sprintf("mp4:%d:source", scaleH),
		scaleH,
		quality,
		"source",
		task.NormalizedSourceStorageName(),
		dbResolution,
		highBitrate || dbResolution == enum.Resolution2160P,
		true,
		true, // KeepSource：null 滤镜，绝不上采样
		task,
	)
	if !highBitrate {
		target.MaxrateUseSource = true
	}

	return []TranscodeTarget{target}
}

// BuildMP4SuperTargets 构造 MP4 多输出目标集合：可选的源片规范化 + 高码率 1080 双档 + 阶梯档。
func BuildMP4SuperTargets(
	task TranscodeTask,
	pixelHeight uint16,
	classHeight uint16,
	sourceResolution enum.VideoResolution,
	resolutions []uint16,
	highBitrate bool,
	needsSource bool,
) []TranscodeTarget {
	targets := make([]TranscodeTarget, 0, len(resolutions)+3)
	if needsSource {
		targets = append(targets, BuildSourceNormalizeTargets(
			task, pixelHeight, classHeight, sourceResolution, highBitrate,
		)...)
	}

	// 是否「够得上」1080 阶梯：用真实高度判断，避免超宽片被当成满血 4K。
	if highBitrate && pixelHeight > 1080 {
		targets = append(targets, NewMP4Target(
			"mp4:1080:high", 1080, QualityModeHigh, "derived",
			task.FileHash+"_1080p_high.mp4", enum.Resolution1080PHigh, true, false, false, task,
		))
	}

	if highBitrate && pixelHeight >= 1080 {
		targets = append(targets, NewMP4Target(
			"mp4:1080:medium", 1080, QualityModeMedium, "derived",
			task.FileHash+"_1080p.mp4", enum.Resolution1080P, false, false, pixelHeight == 1080, task,
		))
	}

	seen := make(map[uint16]struct{}, len(resolutions))

	for _, resolution := range resolutions {
		if resolution == 0 {
			continue
		}
		// 禁止上采样：目标高度不得大于源像素高。
		if pixelHeight > 0 && resolution > pixelHeight {
			continue
		}

		if _, ok := seen[resolution]; ok {
			continue
		}

		seen[resolution] = struct{}{}
		// 与源同高：KeepSource，避免无意义的 scale 损伤。
		keep := pixelHeight > 0 && resolution == pixelHeight
		fileName := fmt.Sprintf("%s_%dp.mp4", task.FileHash, resolution)
		targets = append(targets, NewMP4Target(
			fmt.Sprintf("mp4:%d:standard", resolution),
			resolution,
			QualityModeNone,
			"derived",
			fileName,
			MapResolutionEnumByHeight(int(resolution)),
			resolution >= 1600 || resolution == 2160,
			false,
			keep,
			task,
		))
	}

	return targets
}

// BuildDashResolutions 计算 DASH 阶梯：配置/target 中严格低于真实像素高的档 + 始终追加原生顶档。
// pixelHeight 为 ffprobe 真实高度；禁止出现 2160 配置把 1632 源上采样。
func (p *TranscodePolicy) BuildDashResolutions(targetRes []uint16, pixelHeight uint16) []uint16 {
	set := make(map[uint16]struct{})

	addBelowNative := func(r uint16) {
		if r == 0 {
			return
		}

		if pixelHeight > 0 && r >= pixelHeight {
			return // 顶档单独追加，避免重复/上采样
		}

		set[r] = struct{}{}
	}

	if p != nil && len(p.TranscodeResolutions) > 0 {
		for _, r := range p.TranscodeResolutions {
			addBelowNative(r)
		}
	} else {
		for _, r := range targetRes {
			addBelowNative(r)
		}
	}

	// 原生顶档：保持源像素尺寸（KeepSource），绝不 scale 到更高。
	if pixelHeight > 0 {
		set[pixelHeight] = struct{}{}
	}

	out := make([]uint16, 0, len(set))
	for r := range set {
		out = append(out, r)
	}

	slices.Sort(out)

	return out
}

// BuildDashTargets 构造 DASH 各 representation 目标。
//
// pixelHeight=真实帧高；classHeight=档位分类高（用于画质档位选择）。
// 非高码率原生顶档：KeepSource + remux maxrate；高码率 4K/1080 仍走 High/Medium。
func BuildDashTargets(dashRes []uint16, pixelHeight, classHeight uint16, isHighBitrateSource bool) []TranscodeTarget {
	targets := make([]TranscodeTarget, 0, len(dashRes)+1)
	has1080 := false

	qualityHeight := classHeight
	if qualityHeight == 0 {
		qualityHeight = pixelHeight
	}

	for _, r := range dashRes {
		if r == 0 {
			continue
		}

		if pixelHeight > 0 && r > pixelHeight {
			continue
		}

		target := TranscodeTarget{
			ID:         fmt.Sprintf("dash:%d:standard", r),
			Format:     TranscodePipelineDASH,
			Role:       "derived",
			Resolution: r,
		}

		isNativeTop := pixelHeight > 0 && r == pixelHeight
		if isNativeTop {
			target.KeepSource = true // null 滤镜，不上采样
		}

		switch {
		case isNativeTop && isHighBitrateSource && (r >= 1600 || qualityHeight >= 2160):
			target.ID = "dash:2160:high"
			target.Quality = QualityModeHigh
		case r == 1080 && isHighBitrateSource && !isNativeTop:
			target.ID = "dash:1080:medium"
			target.Quality = QualityModeMedium
			has1080 = true
		case isNativeTop && !isHighBitrateSource:
			// 非高码率顶档：按分类高度选 CRF 档 + remux maxrate（HEVC 抬升）
			target.Quality = QualityForNonHighSourceHeight(qualityHeight)
			target.MaxrateUseSource = true
		case isNativeTop && isHighBitrateSource:
			target.Quality = QualityModeHigh
		}

		if r == 1080 && isHighBitrateSource {
			has1080 = true
		}

		targets = append(targets, target)
	}

	if isHighBitrateSource && has1080 && (pixelHeight == 0 || 1080 < pixelHeight) {
		// 追加 1080p 高码率 representation（QualityModeHigh），与标准 1080P 并存于同一 MPD。
		// 仅当源高于 1080 时追加，避免与原生 1080 顶档重复。
		dup := false

		for _, t := range targets {
			if t.ID == "dash:1080:high" {
				dup = true

				break
			}
		}

		if !dup {
			targets = append(targets, TranscodeTarget{
				ID:         "dash:1080:high",
				Format:     TranscodePipelineDASH,
				Role:       "derived",
				Resolution: 1080,
				Quality:    QualityModeHigh,
			})
		}
	}

	return targets
}

// ValidateSourceTargetIsolation 拒绝会覆盖输入源对象的规范化目标。
func ValidateSourceTargetIsolation(srcPath string, targets []TranscodeTarget) error {
	for _, target := range targets {
		if !target.IsSource {
			continue
		}

		if SourceTargetMatchesInput(srcPath, target.RelativePath) {
			return fmt.Errorf("source normalization target must not overwrite input: %s", target.RelativePath)
		}
	}

	return nil
}

// PipelineResolutionsFromTargets 提取目标集合中首次出现的非零分辨率（进度展示用）。
func PipelineResolutionsFromTargets(targets []TranscodeTarget) []uint16 {
	out := make([]uint16, 0, len(targets))
	seen := make(map[uint16]struct{}, len(targets))

	for _, target := range targets {
		if target.Resolution == 0 {
			continue
		}

		if _, ok := seen[target.Resolution]; ok {
			continue
		}

		seen[target.Resolution] = struct{}{}
		out = append(out, target.Resolution)
	}

	return out
}

// UniqueResolutions 保留首次出现的非零分辨率（进度展示用）。
func UniqueResolutions(resolutions []uint16) []uint16 {
	out := make([]uint16, 0, len(resolutions))
	seen := make(map[uint16]struct{}, len(resolutions))

	for _, resolution := range resolutions {
		if resolution == 0 {
			continue
		}

		if _, ok := seen[resolution]; ok {
			continue
		}

		seen[resolution] = struct{}{}
		out = append(out, resolution)
	}

	return out
}
