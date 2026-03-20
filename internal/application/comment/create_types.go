package comment

// CreateCommand 发布评论命令（由 interfaces 层组装，IP 与归属地在 handler 提取）。
type CreateCommand struct {
	UserID    uint
	VideoID   uint
	DynamicID uint
	Content   string
	ParentID  *uint
	AtUserIDs []uint
	Pictures  []string
	IP        string
	Addr      string
}
