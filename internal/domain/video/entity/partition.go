package entity

import videoerrors "f_ccg_video_back_ddd/internal/domain/video/domainerr"

// Partition 视频分区实体。
type Partition struct {
	ID            uint
	Name          string
	Icon          string
	SortOrder     int
	IsActive      bool
	IsSubmittable bool
}

// EnsureSubmittable 校验分区是否可投稿。
//
// 错误顺序未启用 → 「选择的分区未启用」；不允许投稿 → 「该分区暂不支持投稿」。
func (p *Partition) EnsureSubmittable() error {
	if p == nil {
		return videoerrors.ErrPartitionNotFound
	}

	if !p.IsActive {
		return videoerrors.ErrPartitionInactive
	}

	if !p.IsSubmittable {
		return videoerrors.ErrPartitionNotSubmittable
	}

	return nil
}
