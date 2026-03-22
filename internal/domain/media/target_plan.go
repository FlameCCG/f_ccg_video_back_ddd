package media

import (
	"fmt"
	"path"
	"slices"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// OutputPlanner 构造 MP4 / DASH 输出目标，并执行禁止上采样的阶梯过滤。
// Policy 只提供站点分辨率阶梯；nil 策略与「未配置转码」一样只使用调用方传入的阶梯。
type OutputPlanner struct {
	Policy *TranscodePolicy
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
func (p OutputPlanner) SelectResolutions(resList []uint16, maxHeight int) []uint16 {
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
func (p OutputPlanner) BuildSourceNormalizeTargets(
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
		quality = p.QualityForNonHighSourceHeight(qualityHeight)
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
func (p OutputPlanner) BuildMP4SuperTargets(
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
		targets = append(targets, p.BuildSourceNormalizeTargets(
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

	seen := make(map[uint16]struct{}, len(resolutions)+1)
	sourceClassHeight := (VideoMeta{}).canonicalHeight(sourceResolution)
	if sourceClassHeight > 0 {
		// 原文件已承担其归属档位：例如可播 496p 源直接作为 360p，不再重复编码 360p MP4。
		seen[sourceClassHeight] = struct{}{}
	}

	for _, target := range targets {
		if target.IsSource && target.Resolution > 0 {
			seen[target.Resolution] = struct{}{}
		}
	}

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
			resolution, QualityModeNone, "derived",
			fileName, p.MapResolutionEnumByHeight(int(resolution)), resolution >= 1600 || resolution == 2160,
			false,
			keep,
			task,
		))
	}

	return targets
}

// BuildDashTargets 构造 DASH 各 representation 目标。
//
// pixelHeight=真实帧高；classHeight=档位分类高（用于画质档位选择）。
// 标准档位恰好等于源高时 KeepSource；按宽度归类的超宽 4K 顶档同样保持源像素。
func (p OutputPlanner) BuildDashTargets(dashRes []uint16, pixelHeight, classHeight uint16, isHighBitrateSource bool) []TranscodeTarget {
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

		isNativeTop := p.KeepDASHSourcePixels(r, pixelHeight, classHeight)
		if pixelHeight > 0 && r > pixelHeight && !isNativeTop {
			continue
		}

		target := TranscodeTarget{
			ID:         fmt.Sprintf("dash:%d:standard", r),
			Format:     TranscodePipelineDASH,
			Role:       "derived",
			Resolution: r,
		}

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
			target.Quality = p.QualityForNonHighSourceHeight(qualityHeight)
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

// MapResolutionEnumByHeight 按目标高度映射数据库分辨率枚举。
func (p OutputPlanner) MapResolutionEnumByHeight(height int) enum.VideoResolution {
	if height >= 2160 {
		return enum.Resolution2160P
	}

	if height >= 1080 {
		return enum.Resolution1080P
	}

	if height >= 720 {
		return enum.Resolution720P
	}

	return enum.Resolution360P
}

// QualityForNonHighSourceHeight 非高码率源片/同源高档的画质档位。
// 2160→High，1080→Medium，其余→None（按分辨率 crf720/360）。
func (p OutputPlanner) QualityForNonHighSourceHeight(sourceHeight uint16) QualityMode {
	switch {
	case sourceHeight >= 2160:
		return QualityModeHigh
	case sourceHeight >= 1080:
		return QualityModeMedium
	default:
		return QualityModeNone
	}
}

// DASHTopResolutionHeight 返回源对应的最高标准 DASH 档位。
// 低于最低 360p 档的源保持原高；按宽度归为 4K 的源沿用 MP4 的 2160 分类。
func (p OutputPlanner) DASHTopResolutionHeight(pixelHeight, classHeight uint16) uint16 {
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
func (p OutputPlanner) KeepDASHSourcePixels(resolution, pixelHeight, classHeight uint16) bool {
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

// BuildDashResolutions 计算 DASH 阶梯：非标准源高度向下归入站点支持的标准档位。
// classHeight 与 MP4 的宽高分类一致；按宽度归为 4K 的超宽源保留 2160 档位，
// 实际像素由 KeepDASHSourcePixels 保持不变，绝不把源高上采样到 2160。
func (p OutputPlanner) BuildDashResolutions(
	targetRes []uint16,
	pixelHeight uint16,
	classHeight uint16,
) []uint16 {
	set := make(map[uint16]struct{})
	topHeight := p.DASHTopResolutionHeight(pixelHeight, classHeight)

	addAtOrBelowTop := func(r uint16) {
		if r == 0 {
			return
		}

		if topHeight > 0 && r > topHeight {
			return
		}

		set[r] = struct{}{}
	}

	policy := p.Policy
	if policy != nil && len(policy.TranscodeResolutions) > 0 {
		for _, r := range policy.TranscodeResolutions {
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
