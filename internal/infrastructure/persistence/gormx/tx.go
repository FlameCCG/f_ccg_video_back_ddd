// Package gormx 把本地 GORM 事务绑定到 context 并提供统一取库入口。
//
// 应用层只知道 domain/shared/tx.Manager；TxManager.Do 在此开启事务并向回调传入携带句柄的 context。
// 仓储用 DB(ctx, root) 选择当前事务或根连接，跨仓储原子性依赖每个参与操作都使用同一回调 context。
// 已有事务时 Do 直接复用，不创建嵌套事务或保存点；内层错误若被吞掉，外层不会因此自动回滚。
// Redis、Kafka、对象存储不受该事务控制，相关副作用通常在数据库提交后另行协调。
package gormx

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"

	"gorm.io/gorm"
)

// ctxKeyTx 在 context 中携带当前 GORM 事务句柄的键。
type ctxKeyTx struct{}

// WithTx 把事务句柄放入 context。
func WithTx(ctx context.Context, db *gorm.DB) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if db == nil {
		return ctx
	}

	return context.WithValue(ctx, ctxKeyTx{}, db)
}

// TxFrom 取出 context 中的事务句柄，不存在时返回 nil。
func TxFrom(ctx context.Context) *gorm.DB {
	if ctx == nil {
		return nil
	}

	if db, ok := ctx.Value(ctxKeyTx{}).(*gorm.DB); ok {
		return db
	}

	return nil
}

// DB 返回本次操作应使用的 GORM 句柄：
// context 中存在事务时复用该事务，否则用根连接并绑定 context。
//
// 例如应用在 tx.Manager.Do 的回调中依次调用两个仓储，两者都用 DB(txCtx, root) 才会共用事务。
// 若仓储改用 root，或调用方传回原始 ctx，该操作就可能在事务外提前提交。
func DB(ctx context.Context, root *gorm.DB) *gorm.DB {
	if ctx == nil {
		ctx = context.TODO()
	}

	if txDB := TxFrom(ctx); txDB != nil {
		return txDB
	}

	return root.WithContext(ctx)
}

// TxManager 基于 GORM 的事务编排实现。
type TxManager struct {
	db *gorm.DB
}

// 编译期断言：TxManager 满足领域侧声明的事务端口。
var _ tx.Manager = (*TxManager)(nil)

// NewTxManager 构造事务管理器。
func NewTxManager(db *gorm.DB) *TxManager {
	return &TxManager{db: db}
}

// Do 在一个事务内执行 fn；若 context 中已有事务则直接复用（不开保存点）。
//
// 回调返回 nil 表示可提交，返回 error 触发最外层事务回滚；内层不能独立提交或回滚。
// 事务句柄来自同库 GORM 连接，不应用这个 context 使不相关数据库的仓储共享句柄。
func (m *TxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if ctx == nil {
		ctx = context.TODO()
	}

	if existing := TxFrom(ctx); existing != nil {
		return fn(ctx)
	}

	return m.db.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		return fn(WithTx(ctx, txDB))
	})
}
