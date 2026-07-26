package enum

// ReportStatus 举报处理状态.
type ReportStatus int

// ReportStatusRejected 及相关导出常量.
const (
	// ReportStatusPending 常量.
	ReportStatusPending ReportStatus = iota + 1 // 待处理
	// ReportStatusResolved 常量.
	ReportStatusResolved // 已处理
	// ReportStatusRejected 常量.
	ReportStatusRejected // 已驳回
)
