package video

// PartitionCache 可投稿分区列表的 Redis 缓存端口。
type PartitionCache interface {
	Load() ([]PartitionItem, bool)
	Store(items []PartitionItem)
	Invalidate()
}
