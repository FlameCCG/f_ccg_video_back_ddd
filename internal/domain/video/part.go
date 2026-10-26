package video

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Part 视频分P实体。
type Part struct {
	ID          uint
	VideoID     uint
	Title       string
	SortOrder   int
	Duration    int
	DanmuCount  int64
	StorageType enum.VideoStorageType
}

// ResourceDraft 投稿时写入的初始资源草稿（源片）。
type ResourceDraft struct {
	Resolution     enum.VideoResolution
	SourceFileName string
	FileURL        string
	FileSize       int64
	Bitrate        int
	Format         string
	Codec          string
	IsVip          bool
	IsSource       bool
}

// BuildInitialResourceDraft 组装初始资源草稿；文件路径为空时返回 nil（不写资源行）。
//
// 与原 buildInitialVideoResource / buildInitialPartResource 的字段取值完全一致。
func BuildInitialResourceDraft(fileName, filePath string) *ResourceDraft {
	fileURL := trimSpace(filePath)
	if fileURL == "" {
		return nil
	}

	return &ResourceDraft{
		Resolution:     InitialPublishResolution(),
		SourceFileName: trimSpace(fileName),
		FileURL:        fileURL,
		FileSize:       0,
		Bitrate:        0,
		Format:         InitialPublishFormat(fileName, filePath),
		Codec:          "",
		IsVip:          false,
		IsSource:       true,
	}
}

// PublishDraft 一次投稿需要落库的全部内容（由仓储在单事务内写入）。
type PublishDraft struct {
	// Video 视频主体；仓储写入后回填自增 ID。
	Video *Video
	// PublishAt 与视频主体同时提交；nil 表示无需定时发布。
	PublishAt *time.Time
	// TagIDs 标签关联（为空表示不建立关联）。
	TagIDs []uint
	// Parts 分P列表；仓储写入后逐项回填自增 ID。
	Parts []Part
	// PartResources 与 Parts 同序的初始资源；元素为 nil 表示该分P不写资源行。
	PartResources []*ResourceDraft
	// VideoResource 单视频（非分P）作品的初始资源；nil 表示不写。
	VideoResource *ResourceDraft
}
