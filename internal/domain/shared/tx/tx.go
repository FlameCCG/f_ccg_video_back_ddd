// Package tx 定义应用层可使用的本地事务编排端口。
//
// 应用服务通过 Manager.Do 包住需要共同提交的数据库操作，回调 context 携带实现侧事务句柄。
// 仓储必须继续传递这个 context，基础设施才能复用同一事务；领域和应用层不暴露 *gorm.DB。
// 当前 GORM 实现复用已有事务而不创建保存点；内层返回错误需向外传播才会触发外层回滚。
// 这个端口不使 Redis、Kafka、对象存储或其他数据库自动加入事务。
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
