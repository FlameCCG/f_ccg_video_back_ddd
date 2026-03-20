package identity

import (
	"context"
)

// CoinRecordQuery 硬币流水查询端口（读侧）。
type CoinRecordQuery interface {
	List(ctx context.Context, userID uint, page, pageSize int) ([]CoinRecordItem, int64, error)
}

// ExpRecordQuery 经验流水查询端口（读侧）。
type ExpRecordQuery interface {
	List(ctx context.Context, userID uint, page, pageSize int) ([]ExpRecordItem, int64, error)
}

// LoginRecordQuery 登录记录查询端口（读侧）。
type LoginRecordQuery interface {
	List(ctx context.Context, userID uint, page, pageSize int) ([]LoginRecordItem, int64, error)
}
