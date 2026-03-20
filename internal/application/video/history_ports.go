package video

import (
	"context"
)

// HistoryQuery 观看历史列表读模型查询端口。
type HistoryQuery interface {
	List(ctx context.Context, userID uint, page, pageSize int, keyword string) ([]HistoryItem, int64, error)
}
