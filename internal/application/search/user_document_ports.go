package search

import (
	"context"
	"time"
)

// UserSearchDocument 是写入 search_users 索引的用户文档（字段名由适配器映射为索引字段）.
type UserSearchDocument struct {
	ID              uint
	Username        string
	Description     string
	Avatar          string
	Level           int
	CreatedAt       time.Time
	UpdatedAt       time.Time
	FollowerCount   int64
	FollowUsernames []string
	FriendUsernames []string
}

// UserDocumentIndexer 用户搜索文档写入端口.
type UserDocumentIndexer interface {
	// Available 报告 ES 客户端是否就绪；false 时同步直接跳过。
	Available() bool
	// BulkIndexUsers 批量覆盖写入用户文档。
	BulkIndexUsers(ctx context.Context, docs []UserSearchDocument) error
}

// UserSearchDocumentQuery 用户搜索文档的数据来源端口（用户行 + 关系统计）.
type UserSearchDocumentQuery interface {
	// LoadDocuments 按给定顺序装载用户文档；库中不存在的 ID 直接跳过。
	LoadDocuments(ctx context.Context, userIDs []uint) ([]UserSearchDocument, error)
}
