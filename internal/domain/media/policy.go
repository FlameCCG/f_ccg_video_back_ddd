package media

// TranscodePolicy 是转码编码策略的值对象：把配置里的画质/码率/编码器旋钮
// 收敛成领域可用的决策输入，使分辨率与码率决策不依赖 infrastructure/config。
//
// nil 指针语义等价于原实现的 `global.Config().Transcode == nil`：
// 所有方法都对 nil 接收者安全，并沿用原实现在配置缺失时的回落值。
type TranscodePolicy struct {
	// MaxWorkers 并发转码 worker 数。
	MaxWorkers int
	// TranscodeResolutions 转码分辨率阶梯（MP4 多分辨率与 DASH 共用）。
	TranscodeResolutions []uint16
	// HighBitrateThreshold 触发高码率判定的码率阈值(kbps)。
	HighBitrateThreshold int

	// Bitrate2160HighKbpsReduceRatio 高码率档 4K 的 maxrate 削减比例(%)。
	Bitrate2160HighKbpsReduceRatio int
	// Bitrate1080HighKbpsReduceRatio 高码率档 1080 的 maxrate 削减比例(%)。
	Bitrate1080HighKbpsReduceRatio int
	// Bitrate2160KbpsReduceRatio 标准/中码率档 4K 的 maxrate 削减比例(%)。
	Bitrate2160KbpsReduceRatio int
	// Bitrate1080KbpsReduceRatio 标准/中码率档 1080P 的 maxrate 削减比例(%)。
	Bitrate1080KbpsReduceRatio int
	// Bitrate720KbpsReduceRatio 标准档 720P 的 maxrate 削减比例(%)。
	Bitrate720KbpsReduceRatio int
	// Bitrate360KbpsReduceRatio 标准档 360P 的 maxrate 削减比例(%)。
	Bitrate360KbpsReduceRatio int

	// CrfHigh QualityModeHigh 的 CPU CRF。
	CrfHigh int
	// CrfMedium QualityModeMedium 的 CPU CRF。
	CrfMedium int
	// Crf720 标准档目标分辨率 >=720 的 CPU CRF。
	Crf720 int
	// Crf360 标准档目标分辨率 <=360 的 CPU CRF。
	Crf360 int
	// GpuCQPHigh QualityModeHigh 的 GPU CQP。
	GpuCQPHigh int
	// GpuCQPMedium QualityModeMedium 的 GPU CQP。
	GpuCQPMedium int
	// GpuCQP720 标准档目标分辨率 >=720 的 GPU CQP。
	GpuCQP720 int
	// GpuCQP360 标准档目标分辨率 <=360 的 GPU CQP。
	GpuCQP360 int

	// CpuMode 是否允许 CPU 编码。
	CpuMode bool
	// GpuMode 是否允许 GPU 编码。
	GpuMode bool
	// CpuPreset CPU 编码速度预设。
	CpuPreset string
	// GpuPreset GPU 编码速度预设。
	GpuPreset string
	// Threads ffmpeg 线程数（<=0 表示不显式指定）。
	Threads int
	// HardwareScale 是否启用 GPU 硬件缩放。
	HardwareScale bool

	// Mp4Enable 是否产出 MP4 多分辨率资源。
	Mp4Enable bool
	// DashEnable 是否产出 DASH 自适应流。
	DashEnable bool
	// DashSegDuration DASH 分段时长(秒)，<=0 时按 4 秒。
	DashSegDuration int

	// CheckpointDir 断点续传根目录（空表示系统临时目录）。
	CheckpointDir string
	// CheckpointDisable 关闭断点续传。
	CheckpointDisable bool
	// ChunkDurationSec 分段转码块长(秒)：0=默认 180，<0=关闭分段。
	ChunkDurationSec int
}

// QualityMode 表示转码质量控制策略。
type QualityMode int

const (
	// QualityModeNone 按目标分辨率选 CRF/CQP（标准档）。
	QualityModeNone QualityMode = 0
	// QualityModeHigh 使用 GpuCQPHigh / CrfHigh。
	QualityModeHigh QualityMode = 1
	// QualityModeMedium 使用 GpuCQPMedium / CrfMedium。
	QualityModeMedium QualityMode = 2
)

// 画质档位默认值（配置缺失时的回落，与原实现逐一对应）。
const (
	defaultCQPHigh         = 18
	defaultGpuCQPMedium    = 22
	defaultCrfMedium       = 23
	defaultCQP720          = 22
	defaultCQP360          = 26
	default2160HighReduce  = 12
	default1080HighReduce  = 15
	defaultDashSegDuration = 4
	defaultGpuPreset       = "p4"
	defaultCpuPreset       = "medium"
)

// HighBitrateThresholdOrDefault 返回高码率判定阈值（未配置时 10000 kbps）。
func (p *TranscodePolicy) HighBitrateThresholdOrDefault() int {
	if p != nil && p.HighBitrateThreshold > 0 {
		return p.HighBitrateThreshold
	}

	return defaultHighBitrateThreshold
}

// SegDurationOrDefault 返回 DASH 分段时长（未配置时 4 秒）。
func (p *TranscodePolicy) SegDurationOrDefault() int {
	if p != nil && p.DashSegDuration > 0 {
		return p.DashSegDuration
	}

	return defaultDashSegDuration
}

// PresetOrDefault 返回编码速度预设（GPU 默认 p4，CPU 默认 medium）。
func (p *TranscodePolicy) PresetOrDefault(useGPU bool) string {
	if useGPU {
		if p != nil && p.GpuPreset != "" {
			return p.GpuPreset
		}

		return defaultGpuPreset
	}

	if p != nil && p.CpuPreset != "" {
		return p.CpuPreset
	}

	return defaultCpuPreset
}

// ThreadsOrZero 返回 ffmpeg 线程数；<=0 表示不显式指定。
func (p *TranscodePolicy) ThreadsOrZero() int {
	if p != nil && p.Threads > 0 {
		return p.Threads
	}

	return 0
}

// UseHardwareScale 仅在 GPU 编码且配置开启时启用硬件缩放。
func (p *TranscodePolicy) UseHardwareScale(useGPU bool) bool {
	return useGPU && p != nil && p.HardwareScale
}

// MaxWorkersOrDefault 返回转码 worker 池容量（未配置时 2）。
func (p *TranscodePolicy) MaxWorkersOrDefault() int {
	if p != nil && p.MaxWorkers > 0 {
		return p.MaxWorkers
	}

	return 2
}

// CalcReducedBitrate 按百分比削减源码率；reduceRatio>=100 时按 95 处理。
func CalcReducedBitrate(srcKbps, reduceRatio int) int {
	if srcKbps <= 0 {
		return 0
	}

	if reduceRatio <= 0 {
		return srcKbps
	}

	if reduceRatio >= 100 {
		reduceRatio = 95
	}

	return srcKbps * (100 - reduceRatio) / 100
}

// ResolutionQualityFloorKbps 各档 H264 观感保底（kbps）。仅作 maxrate 下限，不是强制目标码率。
func ResolutionQualityFloorKbps(res uint16) int {
	switch {
	case res >= 1600:
		return 12000 // 4K / 超宽顶档：HEVC 6Mbps 转 H264 时原先 4Mbps 保底过低
	case res >= 1080:
		return 4500
	case res >= 720:
		return 2200
	case res >= 360:
		return 800
	default:
		return 400
	}
}

// NormalizeVideoCodec 归一化 ffprobe 编码名，便于判断高效源。
func NormalizeVideoCodec(codec string) string {
	c := lowerTrim(codec)
	switch c {
	case "h265", "hev1", "hvc1":
		return "hevc"
	case "avc", "avc1", "h.264":
		return "h264"
	default:
		return c
	}
}

// MaxrateForBrowserRemux 浏览器可播重编码（多为 →H264）的 maxrate 预算。
// 高效源（HEVC/AV1/VP9）同码率转 H264 会明显糊，故抬升预算并套档位保底。
func MaxrateForBrowserRemux(srcKbps int, scaleHeight uint16, sourceCodec string) int {
	if srcKbps <= 0 {
		return 0
	}

	out := srcKbps

	switch NormalizeVideoCodec(sourceCodec) {
	case "hevc", "av1", "vp9":
		// 经验：H264 达到接近 HEVC 观感约需 +50%~80% 码率
		out = srcKbps * 170 / 100
	}

	floor := ResolutionQualityFloorKbps(scaleHeight)
	if out < floor {
		out = floor
	}

	// 源极低时不要用满 12Mbps 保底硬拉体积：硬顶约 3× 源。
	if limit := srcKbps * 3; out > limit {
		out = limit
	}

	return out
}

// CalcOutputBitrateForRes 按目标分辨率对源码率做标准档削减（含观感保底保护）。
// 策略缺失或源码率未知时返回 0（等价原 cfg == nil 分支）。
func (p *TranscodePolicy) CalcOutputBitrateForRes(srcBitrate int, res uint16) int {
	if p == nil || srcBitrate <= 0 {
		return 0
	}

	var (
		calculated int
		minLimit   int
	)

	// 保底按「H264 观感」抬高：过低会糊；源更低时仍退回源码率（见下方保护）。
	switch {
	case res >= 1600: // 真 4K / 超宽顶档
		calculated = CalcReducedBitrate(srcBitrate, p.Bitrate2160KbpsReduceRatio)
		minLimit = ResolutionQualityFloorKbps(res)
	case res >= 1080:
		calculated = CalcReducedBitrate(srcBitrate, p.Bitrate1080KbpsReduceRatio)
		minLimit = ResolutionQualityFloorKbps(res)
	case res >= 720:
		calculated = CalcReducedBitrate(srcBitrate, p.Bitrate720KbpsReduceRatio)
		minLimit = ResolutionQualityFloorKbps(res)
	case res >= 360:
		calculated = CalcReducedBitrate(srcBitrate, p.Bitrate360KbpsReduceRatio)
		minLimit = ResolutionQualityFloorKbps(res)
	default:
		return 0
	}

	// 保护逻辑：如果削减后的码率低于保底码率
	if calculated < minLimit {
		// 如果源视频本身码率就低于保底码率，则直接不作削减限制，采用源视频的码率
		if srcBitrate <= minLimit {
			return srcBitrate
		}
		// 否则，使用保底码率限制
		return minLimit
	}

	return calculated
}

// HighQualityReduceRatio 返回 QualityModeHigh 的 maxrate 削减比例。
// 4K(>=2160) 用 bitrate2160HighKbpsReduceRatio，其余 High 档用 bitrate1080HighKbpsReduceRatio。
func (p *TranscodePolicy) HighQualityReduceRatio(res uint16) int {
	if res >= 2160 {
		if p != nil && p.Bitrate2160HighKbpsReduceRatio > 0 {
			return p.Bitrate2160HighKbpsReduceRatio
		}

		return default2160HighReduce
	}

	if p != nil && p.Bitrate1080HighKbpsReduceRatio > 0 {
		return p.Bitrate1080HighKbpsReduceRatio
	}

	return default1080HighReduce
}

// OutputCQP 返回标准档(QualityModeNone)各目标分辨率的 CRF(CPU)/CQP(GPU)。
// 分辨率 >=720（含普通 1080/720）用 crf720/gpuCQP720；<=360 用 crf360/gpuCQP360。
// 与源是否高码率无关；码率封顶由 -maxrate（reduce 配置）另行控制。
func (p *TranscodePolicy) OutputCQP(res uint16, gpu bool) int {
	if res >= 720 {
		if gpu {
			if p != nil && p.GpuCQP720 > 0 {
				return p.GpuCQP720
			}

			return defaultCQP720
		}

		if p != nil && p.Crf720 > 0 {
			return p.Crf720
		}

		return defaultCQP720
	}
	// 360P 及以下
	if gpu {
		if p != nil && p.GpuCQP360 > 0 {
			return p.GpuCQP360
		}

		return defaultCQP360
	}

	if p != nil && p.Crf360 > 0 {
		return p.Crf360
	}

	return defaultCQP360
}

// OutputCQPForTarget 根据 target 的 Quality 返回 CRF(CPU)/CQP(GPU) 质量值。
// QualityModeHigh → 高码率档；QualityModeMedium → 中码率档；默认 → 按分辨率。
func (p *TranscodePolicy) OutputCQPForTarget(t TranscodeTarget, gpu bool) int {
	switch t.Quality {
	case QualityModeHigh:
		if gpu {
			if p != nil && p.GpuCQPHigh > 0 {
				return p.GpuCQPHigh
			}

			return defaultCQPHigh
		}

		if p != nil && p.CrfHigh > 0 {
			return p.CrfHigh
		}

		return defaultCQPHigh
	case QualityModeMedium:
		if gpu {
			if p != nil && p.GpuCQPMedium > 0 {
				return p.GpuCQPMedium
			}

			return defaultGpuCQPMedium
		}

		if p != nil && p.CrfMedium > 0 {
			return p.CrfMedium
		}

		return defaultCrfMedium
	default:
		return p.OutputCQP(t.Resolution, gpu)
	}
}

// OutputBitrateForTarget 根据 target 返回 -maxrate 码率上限(kbps)。
//
// MaxrateUseSource → 浏览器 remux 预算（高效源抬升 + 档位保底）；
// QualityModeHigh → High reduce 配置 + 观感保底；
// QualityModeMedium / 默认 → 按分辨率标准 reduce。
// sourceCodec 来自 ffprobe（hevc/h264/…），仅 MaxrateUseSource / 保底策略使用。
func (p *TranscodePolicy) OutputBitrateForTarget(t TranscodeTarget, srcBitrate int, sourceCodec string) int {
	if srcBitrate <= 0 {
		return 0
	}

	if t.MaxrateUseSource {
		return MaxrateForBrowserRemux(srcBitrate, t.Resolution, sourceCodec)
	}

	if p == nil {
		return 0
	}

	switch t.Quality {
	case QualityModeHigh:
		// 高码率源 High 档：reduce 后仍不低于观感保底（且不超过源过多由 reduce 保证）
		v := CalcReducedBitrate(srcBitrate, p.HighQualityReduceRatio(t.Resolution))
		floor := ResolutionQualityFloorKbps(t.Resolution)

		if v > 0 && v < floor && srcBitrate > floor {
			return floor
		}

		if v > 0 && v < floor && srcBitrate <= floor {
			return srcBitrate
		}

		return v
	case QualityModeMedium:
		return p.CalcOutputBitrateForRes(srcBitrate, t.Resolution)
	default:
		return p.CalcOutputBitrateForRes(srcBitrate, t.Resolution)
	}
}

// QualityForNonHighSourceHeight 非高码率源片/同源高档的画质档位。
// 2160→High，1080→Medium，其余→None（按分辨率 crf720/360）。
func QualityForNonHighSourceHeight(sourceHeight uint16) QualityMode {
	switch {
	case sourceHeight >= 2160:
		return QualityModeHigh
	case sourceHeight >= 1080:
		return QualityModeMedium
	default:
		return QualityModeNone
	}
}
