package enum

// ReportStatus 举报处理状态.
type ReportStatus int

const (
	// ReportStatusPending 表示等待管理端处理的举报。
	ReportStatusPending ReportStatus = iota + 1 // 待处理
	// ReportStatusResolved 表示已完成处理的举报。
	ReportStatusResolved // 已处理
	// ReportStatusRejected 表示被驳回的举报。
	ReportStatusRejected // 已驳回
)
