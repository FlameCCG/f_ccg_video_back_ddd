package media

import (
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
