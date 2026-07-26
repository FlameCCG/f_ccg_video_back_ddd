// Package tx 定义跨仓储事务编排的领域端口。
//
// 原项目通过在服务方法签名里直接传 *gorm.DB（如 CoinService.ApplyChangeTx）
// 来实现跨模块共享事务，使调用方被迫依赖 GORM。
// 新架构改为：应用层通过 tx.Manager 开启事务，事务句柄随 context 向下传递，
// 各仓储实现从 context 取出当前事务；领域层与应用层因此完全不感知 GORM。
package tx

import "context"

// Manager 事务编排端口，由 infrastructure 的 GORM 实现提供。
type Manager interface {
	// Do 在一个事务内执行 fn。fn 收到的 ctx 携带事务句柄，
	// 其中调用的所有仓储方法都会落在同一事务上。
	//
	// 若 ctx 中已存在事务（嵌套调用），实现应直接复用该事务而不是开启保存点，
	// 语义与原项目「显式传 tx 的 XxxTx 方法」一致。
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
