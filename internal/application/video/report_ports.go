package video

import (
	"context"
)

// ReportQuery 管理端举报列表读模型查询端口。
type ReportQuery interface {
	AdminVideoReports(ctx context.Context, page, pageSize, status int) ([]AdminVideoReportItem, int64, error)
	AdminDanmuReports(ctx context.Context, page, pageSize, status int) ([]AdminDanmuReportItem, int64, error)
}
