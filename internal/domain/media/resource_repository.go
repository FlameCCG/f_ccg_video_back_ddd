package media

import (
	"context"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// TranscodeResourceRepository 转码任务的数据库读写端口。
//
// 读写 videos / video_parts / video_resources / video_part_resources 表，实现在 infrastructure/persistence/media。
type TranscodeResourceRepository interface {
	// EnsureTaskActive 校验视频（及分P）仍存在且未被删除；已删除时返回 ErrVideoDeleted。
	EnsureTaskActive(ctx context.Context, task TranscodeTask) error
	// UpdateDurationFromProbe 用探测到的时长回填分P/视频时长。
	UpdateDurationFromProbe(ctx context.Context, task TranscodeTask, duration int) error
	// SaveOriginalResource 落库源视频资源行（同分辨率去重、保留已规范化产物 URL）。
	SaveOriginalResource(
		ctx context.Context,
		task TranscodeTask,
		meta VideoMeta,
		sourceURL string,
		size int64,
		highBitrate bool,
	) error
	// UpdateOriginalResourceFile 把源资源行指向规范化后的产物。
	UpdateOriginalResourceFile(
		ctx context.Context,
		task TranscodeTask,
		res enum.VideoResolution,
		newFileName, url string,
		size int64,
		bitrate int,
	) error
	// SaveVideoResource 按输出高度 upsert 一条派生资源行。
	SaveVideoResource(
		ctx context.Context,
		task TranscodeTask,
		res uint16,
		sourceName, url string,
		size int64,
		bitrate int,
	) error
	// SaveSpecificVideoResource 按显式分辨率枚举 upsert 一条资源行。
	SaveSpecificVideoResource(
		ctx context.Context,
		task TranscodeTask,
		res enum.VideoResolution,
		sourceName, url string,
		size int64,
		bitrate int,
		isVip, isSource bool,
	) error
	// SaveDASHResource 把整套 DASH 落库为一行自适应资源。
	SaveDASHResource(ctx context.Context, task TranscodeTask, sourceName, mpdURL string, totalSize int64) error
	// PersistedResourceURLs 返回匹配 (resolution, isSource) 的资源行 file_url 列表；
	// resolution 为空串表示不按分辨率过滤（源行查询用）。
	PersistedResourceURLs(
		ctx context.Context,
		task TranscodeTask,
		resolution enum.VideoResolution,
		isSource bool,
	) ([]string, error)
}
