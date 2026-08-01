package logger

import "github.com/google/wire"

// ProviderSet 汇总日志基础设施客户端及其底层 Zap 实例。
var ProviderSet = wire.NewSet(
	Provide,
	ProvideZap,
)
