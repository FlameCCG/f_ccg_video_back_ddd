package contentops

// TouchResult 站点打点结果（读模型）.
type TouchResult struct {
	PV     int64 `json:"pv"`
	UV     int64 `json:"uv"`
	Online int64 `json:"online"`
}

// HeartbeatResult 在线心跳结果（读模型）.
type HeartbeatResult struct {
	UV     int64 `json:"uv"`
	Online int64 `json:"online"`
}
