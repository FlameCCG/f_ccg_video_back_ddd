package video

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
// 错误顺序与原 persistVideo / UpdateVideo 中的校验一致：
// 未启用 → 「选择的分区未启用」；不允许投稿 → 「该分区暂不支持投稿」。
func (p *Partition) EnsureSubmittable() error {
	if p == nil {
		return ErrPartitionNotFound
	}

	if !p.IsActive {
		return ErrPartitionInactive
	}

	if !p.IsSubmittable {
		return ErrPartitionNotSubmittable
	}

	return nil
}
