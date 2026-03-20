package video

import (
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// PublishVideoCommand 投稿命令（handler 校验绑定后交由应用层执行）。
type PublishVideoCommand struct {
	AuthorID       uint
	Title          string
	Description    string
	PartitionID    uint
	TagIDs         []uint
	IsOriginal     bool
	IsPrivate      bool
	CoverURL       string
	FilePath       string
	FileName       string
	FileHash       string
	Parts          []PublishPartInput
	PublishTime    time.Time
	AuthorUsername string
}

// PublishPartInput 投稿分P输入。
type PublishPartInput struct {
	Title    string
	FilePath string
	FileName string
	FileHash string
}

// PublishVideoResult 投稿结果。
type PublishVideoResult struct {
	VideoID     uint
	Status      enum.VideoStatus
	PublishTime time.Time
	Cover       string
}
