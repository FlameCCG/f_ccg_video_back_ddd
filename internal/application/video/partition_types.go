package video

// PartitionItem 分区列表项。
type PartitionItem struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Icon          string `json:"icon"`
	SortOrder     int    `json:"sortOrder"`
	IsActive      bool   `json:"isActive"`
	IsSubmittable bool   `json:"isSubmittable"`
}
