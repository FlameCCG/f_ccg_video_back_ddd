// Package auth 提供鉴权基础设施：Casbin enforcer 与 JWT 签发/解析适配。
package auth

import (
	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// casbinModelPath Casbin 模型配置文件路径（与原实现一致，相对工作目录）。
const casbinModelPath = "conf/rbac_model.conf"

// NewEnforcer 基于 GORM adapter 构建 Casbin SyncedEnforcer 并加载策略。
// 语义与原 core.InitCasbin 一致：任一步失败均 zap.Fatal。
func NewEnforcer(db *gorm.DB) *casbin.SyncedEnforcer {
	// 使用 GORM adapter
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		zap.L().Fatal("初始化 Casbin adapter 失败", zap.Error(err))
	}

	// 加载模型配置文件（matchers 使用内置 globMatch，无需自定义 pathMatch）
	enforcer, err := casbin.NewSyncedEnforcer(casbinModelPath, adapter)
	if err != nil {
		zap.L().Fatal("初始化 Casbin enforcer 失败", zap.Error(err))
	}

	// 加载策略
	if err := enforcer.LoadPolicy(); err != nil {
		zap.L().Fatal("加载 Casbin 策略失败", zap.Error(err))
	}

	return enforcer
}
