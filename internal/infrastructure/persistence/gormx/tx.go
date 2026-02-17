// Package gormx 提供 GORM 事务在 context 中的传递与仓储取库助手。
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
