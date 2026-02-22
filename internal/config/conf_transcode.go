package config

import "errors"

// Transcode 定义转码相关配置.
type Transcode struct {
	MaxWorkers                     int      `mapstructure:"maxWorkers" json:"maxWorkers"`                                         // 并发转码 worker 数
	TranscodeResolutions           []uint16 `mapstructure:"transcodeResolutions" json:"transcodeResolutions"`                     // 转码分辨率列表（MP4 多分辨率与 DASH 共用）
	HighBitrateThreshold           int      `mapstructure:"highBitrateThreshold" json:"highBitrateThreshold"`                     // 触发高码率判定的码率阈值(kbps)(只有在>=1080P/4K才会生效)
	Bitrate2160HighKbpsReduceRatio int      `mapstructure:"bitrate2160HighKbpsReduceRatio" json:"bitrate2160HighKbpsReduceRatio"` // 高码率档 QualityModeHigh：4K 源片规范化 / DASH 2160 High 的 maxrate 削减比例(百分比)
	Bitrate1080HighKbpsReduceRatio int      `mapstructure:"bitrate1080HighKbpsReduceRatio" json:"bitrate1080HighKbpsReduceRatio"` // 高码率档 QualityModeHigh：1080 源片规范化 / 1080 High 的 maxrate 削减比例(百分比)
	Bitrate2160KbpsReduceRatio     int      `mapstructure:"bitrate2160KbpsReduceRatio" json:"bitrate2160KbpsReduceRatio"`         // 标准/中码率档：4K(2160P) maxrate 相对源码率削减比例(百分比)
	Bitrate1080KbpsReduceRatio     int      `mapstructure:"bitrate1080KbpsReduceRatio" json:"bitrate1080KbpsReduceRatio"`         // 标准/中码率档：1080P maxrate 相对源码率削减比例(百分比)
	Bitrate720KbpsReduceRatio      int      `mapstructure:"bitrate720KbpsReduceRatio" json:"bitrate720KbpsReduceRatio"`           // 标准档：720P maxrate 相对源码率削减比例(百分比，勿>=100)
	Bitrate360KbpsReduceRatio      int      `mapstructure:"bitrate360KbpsReduceRatio" json:"bitrate360KbpsReduceRatio"`           // 标准档：360P maxrate 相对源码率削减比例(百分比，勿>=100)
	CpuMode                        bool     `mapstructure:"cpuMode" json:"cpuMode"`                                               // 是否开启CPU转码（与 GpuMode 至少一个为 true）
	CrfHigh                        int      `mapstructure:"crfHigh" json:"crfHigh"`                                               // QualityModeHigh：源片规范化/1080High/4K High 的 CPU CRF（越小质量越高，推荐18）
	CrfMedium                      int      `mapstructure:"crfMedium" json:"crfMedium"`                                           // QualityModeMedium：高码率 1080 Medium 的 CPU CRF（推荐23）
	Crf720                         int      `mapstructure:"crf720" json:"crf720"`                                                 // 标准档(QualityModeNone)：目标分辨率>=720（含普通1080/720）的 CPU CRF，与是否高码率无关（推荐24）
	Crf360                         int      `mapstructure:"crf360" json:"crf360"`                                                 // 标准档(QualityModeNone)：目标分辨率<=360 的 CPU CRF，与是否高码率无关（推荐28）
	CpuPreset                      string   `mapstructure:"cpuPreset" json:"cpuPreset"`                                           // CPU转码的预设参数(ultrafast, superfast, veryfast, faster, fast, medium, slow, slower, veryslow),越慢压缩效率越好同时画质还能尽量保持,不要用ultrafast或者superfast因为他会忽略CRF的质量设置
	GpuCQPHigh                     int      `mapstructure:"gpuCQPHigh" json:"gpuCQPHigh"`                                         // QualityModeHigh：源片规范化/1080High/4K High 的 GPU CQP（推荐18）
	GpuCQPMedium                   int      `mapstructure:"gpuCQPMedium" json:"gpuCQPMedium"`                                     // QualityModeMedium：高码率 1080 Medium 的 GPU CQP（推荐22）
	GpuCQP720                      int      `mapstructure:"gpuCQP720" json:"gpuCQP720"`                                           // 标准档：目标分辨率>=720（含普通1080/720）的 GPU CQP，与是否高码率无关（推荐24）
	GpuCQP360                      int      `mapstructure:"gpuCQP360" json:"gpuCQP360"`                                           // 标准档：目标分辨率<=360 的 GPU CQP，与是否高码率无关（推荐28）
	GpuPreset                      string   `mapstructure:"gpuPreset" json:"gpuPreset"`                                           // GPU转码的预设参数(p1、p2、p3、p4、p5、p6)
	GpuMode                        bool     `mapstructure:"gpuMode" json:"gpuMode"`                                               // 是否开启GPU转码（与 CpuMode 至少一个为 true）
	Threads                        int      `mapstructure:"threads" json:"threads"`                                               // 转码的线程数
	HardwareScale                  bool     `mapstructure:"hardwareScale" json:"hardwareScale"`                                   // 是否开启硬件缩放
	Mp4Enable                      bool     `mapstructure:"mp4Enable" json:"mp4Enable"`                                           // 是否产出 MP4 多分辨率资源；与 dashEnable 都为 false 时仍会做源片规范化
	DashEnable                     bool     `mapstructure:"dashEnable" json:"dashEnable"`                                         // 是否产出 DASH 自适应流；与 mp4Enable 都为 false 时仍会做源片规范化
	DashSegDuration                int      `mapstructure:"dashSegDuration" json:"dashSegDuration"`                               // DASH 分段时长(秒)，默认 4
	CheckpointDir                  string   `mapstructure:"checkpointDir" json:"checkpointDir"`                                   // 转码断点续传目录（空 → 系统临时目录/transcode-checkpoints）；重试时复用已编码产物避免重复编码
	CheckpointDisable              bool     `mapstructure:"checkpointDisable" json:"checkpointDisable"`                           // 关闭断点续传（每次重试全量重编码），默认开启
	ChunkDurationSec               int      `mapstructure:"chunkDurationSec" json:"chunkDurationSec"`                             // 分段转码块长(秒)：长视频按块编码+按块断点，中断只重编当前块。0=默认180，<0=关闭分段（保留单元级断点）
}

// Validate 校验转码配置的合法性。
func (t *Transcode) Validate() error {
	if t == nil {
		return nil
	}

	if !t.GpuMode && !t.CpuMode {
		return errors.New("GpuMode 和 CpuMode 不能同时为 false，请至少开启一种转码模式")
	}

	return nil
}
