package notification

// NotificationCountSummary 角标计数汇总（读模型）.
type NotificationCountSummary struct {
	Reply   int64 `json:"reply"`
	Like    int64 `json:"like"`
	At      int64 `json:"at"`
	System  int64 `json:"system"`
	Message int64 `json:"message"`
}
