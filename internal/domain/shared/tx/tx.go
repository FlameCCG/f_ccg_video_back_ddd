// Package tx 定义跨仓储事务编排端口。
//
// 应用层通过 tx.Manager 开启事务，句柄随 context 向下传递；仓储从 context 取得当前事务，使领域层与应用层无需依赖 GORM。
package tx

import "context"

// Manager 事务编排端口，由 infrastructure 的 GORM 实现提供。
type Manager interface {
	// Do 在一个事务内执行 fn，传入的 context 携带事务句柄，其中的仓储调用共享此事务。
	//
	// context 已包含事务时直接复用，不开启保存点。
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
