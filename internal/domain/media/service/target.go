package service

import (
	"fmt"
	"path"

	mediaentity "f_ccg_video_back_ddd/internal/domain/media/entity"
	mediavo "f_ccg_video_back_ddd/internal/domain/media/valueobject"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// NewMP4Target 构造一路 MP4 输出目标。
func NewMP4Target(
	id string,
	resolution uint16,
	quality mediavo.QualityMode,
	role, fileName string,
	dbResolution enum.VideoResolution,
	isVIP, isSource, keepSource bool,
	task mediaentity.TranscodeTask,
) mediavo.TranscodeTarget {
	return mediavo.TranscodeTarget{
		ID:           id,
		Format:       mediavo.TranscodePipelineMP4,
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
	task mediaentity.TranscodeTask,
	pixelHeight uint16,
	classHeight uint16,
	sourceResolution enum.VideoResolution,
	highBitrate bool,
) []mediavo.TranscodeTarget {
	dbResolution := sourceResolution
	if highBitrate && dbResolution == enum.Resolution1080P {
		dbResolution = enum.Resolution1080PHigh
	}

	// 画质档位按分类高度（真 4K 档用 High），缩放高度用真实像素。
	qualityHeight := classHeight
	if qualityHeight == 0 {
		qualityHeight = pixelHeight
	}

	quality := mediavo.QualityModeHigh
	if !highBitrate {
		quality = mediavo.QualityForNonHighSourceHeight(qualityHeight)
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

	return []mediavo.TranscodeTarget{target}
}

// BuildMP4SuperTargets 构造 MP4 多输出目标集合：可选的源片规范化 + 高码率 1080 双档 + 阶梯档。
func BuildMP4SuperTargets(
	task mediaentity.TranscodeTask,
	pixelHeight uint16,
	classHeight uint16,
	sourceResolution enum.VideoResolution,
	resolutions []uint16,
	highBitrate bool,
	needsSource bool,
) []mediavo.TranscodeTarget {
	targets := make([]mediavo.TranscodeTarget, 0, len(resolutions)+3)
	if needsSource {
		targets = append(targets, BuildSourceNormalizeTargets(
			task, pixelHeight, classHeight, sourceResolution, highBitrate,
		)...)
	}

	// 是否「够得上」1080 阶梯：用真实高度判断，避免超宽片被当成满血 4K。
	if highBitrate && pixelHeight > 1080 {
		targets = append(targets, NewMP4Target(
			"mp4:1080:high", 1080, mediavo.QualityModeHigh, "derived",
			task.FileHash+"_1080p_high.mp4", enum.Resolution1080PHigh, true, false, false, task,
		))
	}

	if highBitrate && pixelHeight >= 1080 {
		targets = append(targets, NewMP4Target(
			"mp4:1080:medium", 1080, mediavo.QualityModeMedium, "derived",
			task.FileHash+"_1080p.mp4", enum.Resolution1080P, false, false, pixelHeight == 1080, task,
		))
	}

	seen := make(map[uint16]struct{}, len(resolutions)+1)
	sourceClassHeight := mediavo.CanonicalResolutionHeight(sourceResolution)
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
			resolution, mediavo.QualityModeNone, "derived",
			fileName, mediavo.MapResolutionEnumByHeight(int(resolution)), resolution >= 1600 || resolution == 2160,
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
func BuildDashTargets(dashRes []uint16, pixelHeight, classHeight uint16, isHighBitrateSource bool) []mediavo.TranscodeTarget {
	targets := make([]mediavo.TranscodeTarget, 0, len(dashRes)+1)
	has1080 := false

	qualityHeight := classHeight
	if qualityHeight == 0 {
		qualityHeight = pixelHeight
	}

	for _, r := range dashRes {
		if r == 0 {
			continue
		}

		isNativeTop := mediavo.KeepDASHSourcePixels(r, pixelHeight, classHeight)
		if pixelHeight > 0 && r > pixelHeight && !isNativeTop {
			continue
		}

		target := mediavo.TranscodeTarget{
			ID:         fmt.Sprintf("dash:%d:standard", r),
			Format:     mediavo.TranscodePipelineDASH,
			Role:       "derived",
			Resolution: r,
		}

		if isNativeTop {
			target.KeepSource = true // null 滤镜，不上采样
		}

		switch {
		case isNativeTop && isHighBitrateSource && (r >= 1600 || qualityHeight >= 2160):
			target.ID = "dash:2160:high"
			target.Quality = mediavo.QualityModeHigh
		case r == 1080 && isHighBitrateSource && !isNativeTop:
			target.ID = "dash:1080:medium"
			target.Quality = mediavo.QualityModeMedium
			has1080 = true
		case isNativeTop && !isHighBitrateSource:
			// 非高码率顶档：按分类高度选 CRF 档 + remux maxrate（HEVC 抬升）
			target.Quality = mediavo.QualityForNonHighSourceHeight(qualityHeight)
			target.MaxrateUseSource = true
		case isNativeTop && isHighBitrateSource:
			target.Quality = mediavo.QualityModeHigh
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
			targets = append(targets, mediavo.TranscodeTarget{
				ID:         "dash:1080:high",
				Format:     mediavo.TranscodePipelineDASH,
				Role:       "derived",
				Resolution: 1080,
				Quality:    mediavo.QualityModeHigh,
			})
		}
	}

	return targets
}

// ValidateSourceTargetIsolation 拒绝会覆盖输入源对象的规范化目标。
func ValidateSourceTargetIsolation(srcPath string, targets []mediavo.TranscodeTarget) error {
	for _, target := range targets {
		if !target.IsSource {
			continue
		}

		if mediavo.SourceTargetMatchesInput(srcPath, target.RelativePath) {
			return fmt.Errorf("source normalization target must not overwrite input: %s", target.RelativePath)
		}
	}

	return nil
}

// PipelineResolutionsFromTargets 提取目标集合中首次出现的非零分辨率（进度展示用）。
func PipelineResolutionsFromTargets(targets []mediavo.TranscodeTarget) []uint16 {
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
