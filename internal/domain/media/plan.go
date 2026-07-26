package media

import (
	"slices"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// PipelineMode 描述本次任务实际选择的管线组合（供日志区分，与原实现的分支一一对应）。
type PipelineMode string

const (
	// PipelineModeMP4Super mp4Enable 开启：完整 MP4 多输出（可含源片规范化）。
	PipelineModeMP4Super PipelineMode = "mp4_super"
	// PipelineModeSourceOnly mp4Enable 与 dashEnable 均关闭且源不可播：只做源片规范化。
	PipelineModeSourceOnly PipelineMode = "source_only"
	// PipelineModeNone mp4Enable 与 dashEnable 均关闭且源已可播：不做任何重编码。
	PipelineModeNone PipelineMode = "none"
	// PipelineModeDashOnly 仅 dashEnable 开启：只产出 DASH。
	PipelineModeDashOnly PipelineMode = "dash_only"
)

// PipelinePlan 是「一次转码任务要跑哪些管线、每条管线跑哪些目标」的领域决策结果。
type PipelinePlan struct {
	// Mode 本次选择的管线组合。
	Mode PipelineMode
	// SourceResolution 源片的数据库分辨率枚举。
	SourceResolution enum.VideoResolution
	// ClassHeight 档位分类高度（宽≥3840 会标成 2160）。
	ClassHeight uint16
	// PixelHeight 真实帧高，用于 scale，禁止上采样。
	PixelHeight uint16
	// HighBitrate 源片是否为高码率。
	HighBitrate bool
	// BrowserPlayable 源片是否可被浏览器直接播放。
	BrowserPlayable bool
	// NotPlayableReason 源片不可直接播放的原因（可播放时为空串）。
	NotPlayableReason string
	// NeedsSourceMP4 是否需要对源片做浏览器兼容规范化。
	NeedsSourceMP4 bool
	// TargetResolutions 过滤后的阶梯档（严格低于源像素高，高码率 4K 源剔除 1080）。
	TargetResolutions []uint16
	// MP4Targets MP4 管线的输出目标；为空表示不跑 MP4 管线。
	MP4Targets []TranscodeTarget
	// DashResolutions DASH 管线的阶梯；为空表示不跑 DASH 管线。
	DashResolutions []uint16
}

// HasWork 报告是否至少有一条管线需要运行。
func (p PipelinePlan) HasWork() bool {
	return len(p.MP4Targets) > 0 || len(p.DashResolutions) > 0
}

// PlanPipelines 依据源片元数据与转码策略决定要执行的管线与目标集合。
//
// 规则与原 executeConfiguredPipelines 的前半段完全一致：
//   - 阶梯天花板 = 真实像素高，绝不 scale 到比源更高；
//   - 高码率且真实高度 > 1080 时剔除阶梯里的 1080（改由 1080 high/medium 双档承担）；
//   - mp4Enable 与 dashEnable 都关闭时，仍至少做源片规范化（源已可播且非高码率则完全跳过）。
func PlanPipelines(task TranscodeTask, meta VideoMeta, policy *TranscodePolicy) PipelinePlan {
	sourceResolution := meta.Resolution()
	classHeight := CanonicalResolutionHeight(sourceResolution)
	pixelHeight := EffectivePixelHeight(meta.Height, classHeight)
	highBitrate := meta.IsHighBitrate(policy.HighBitrateThresholdOrDefault())
	playable, reason := meta.AssessBrowserPlayableSource()

	plan := PipelinePlan{
		SourceResolution:  sourceResolution,
		ClassHeight:       classHeight,
		PixelHeight:       pixelHeight,
		HighBitrate:       highBitrate,
		BrowserPlayable:   playable,
		NotPlayableReason: reason,
		NeedsSourceMP4:    !playable || highBitrate,
	}

	targetRes := SelectResolutions(task.ResolutionList, int(pixelHeight))
	if highBitrate && pixelHeight > 1080 {
		targetRes = slices.DeleteFunc(targetRes, func(res uint16) bool { return res == 1080 })
	}

	plan.TargetResolutions = targetRes

	mp4Enable := policy != nil && policy.Mp4Enable
	dashEnable := policy != nil && policy.DashEnable

	switch {
	case mp4Enable:
		plan.Mode = PipelineModeMP4Super
		plan.MP4Targets = BuildMP4SuperTargets(
			task, pixelHeight, classHeight, sourceResolution, targetRes, highBitrate, plan.NeedsSourceMP4,
		)
	case !dashEnable:
		if plan.NeedsSourceMP4 {
			plan.Mode = PipelineModeSourceOnly
			plan.MP4Targets = BuildSourceNormalizeTargets(
				task, pixelHeight, classHeight, sourceResolution, highBitrate,
			)
		} else {
			plan.Mode = PipelineModeNone
		}
	default:
		plan.Mode = PipelineModeDashOnly
	}

	if dashEnable {
		plan.DashResolutions = policy.BuildDashResolutions(targetRes, pixelHeight)
	}

	return plan
}
