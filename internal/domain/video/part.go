package video

import (
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// VideoPart 视频分P实体。
type VideoPart struct {
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

// InitialDraft 组装初始资源草稿；文件路径为空时返回 nil（不写资源行）。
// 。
func (ResourcePolicy) InitialDraft(fileName, filePath string) *ResourceDraft {
	fileURL := strings.TrimSpace(filePath)
	if fileURL == "" {
		return nil
	}

	return &ResourceDraft{
		Resolution:     (ResourcePolicy{}).InitialResolution(),
		SourceFileName: strings.TrimSpace(fileName),
		FileURL:        fileURL,
		FileSize:       0,
		Bitrate:        0,
		Format:         (ResourcePolicy{}).InitialFormat(fileName, filePath),
		Codec:          "",
		IsVip:          false,
		IsSource:       true,
	}
}

// PublishDraft 一次投稿需要落库的全部内容（由仓储在单事务内写入）。
type PublishDraft struct {
	// Video 待写入的视频主体，仓储不修改此指针或其状态。
	Video *Video
	// PublishAt 与视频主体同时提交；nil 表示无需定时发布。
	PublishAt *time.Time
	// TagIDs 标签关联（为空表示不建立关联）。
	TagIDs []uint
	// Parts 待写入的分 P 列表，持久化身份通过 PublicationResult 返回。
	Parts []VideoPart
	// PartResources 与 Parts 同序的初始资源；元素为 nil 表示该分P不写资源行。
	PartResources []*ResourceDraft
	// VideoResource 单视频（非分P）作品的初始资源；nil 表示不写。
	VideoResource *ResourceDraft
}
