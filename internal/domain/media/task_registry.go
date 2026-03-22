package media

// TaskKey 标识一次属于某个视频或分P 的转码运行。
type TaskKey struct {
	// VideoID 视频 ID。
	VideoID uint
	// PartID 分P ID；0 表示单P视频。
	PartID uint
}
