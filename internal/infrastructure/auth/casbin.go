// Package auth 构造 Casbin 鉴权引擎并提供认证工具的装配入口。
//
// casbin.go 创建数据库策略适配器，provider.go 向组合根导出构造能力。
// 领域授权规则经端口使用引擎，HTTP 中间件负责取得当前用户与路由，再执行请求级鉴权。
//
// 具体对象由 bootstrap 构造并通过端口交给内层，技术实现不应反过来要求业务层导入本包。
package auth

import (
	"fmt"

	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

// casbinModelPath Casbin 模型配置文件路径（与原实现一致，相对工作目录）。
const casbinModelPath = "conf/rbac_model.conf"

// NewEnforcer 基于 GORM adapter 构建 Casbin SyncedEnforcer 并加载策略。
// 初始化失败向组合根返回错误，由 CLI/server 入口决定退出策略。
func NewEnforcer(db *gorm.DB) (*casbin.SyncedEnforcer, error) {
	// 使用 GORM adapter
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, fmt.Errorf("初始化 Casbin adapter 失败: %w", err)
	}

	// 加载模型配置文件（matchers 使用内置 globMatch，无需自定义 pathMatch）
	enforcer, err := casbin.NewSyncedEnforcer(casbinModelPath, adapter)
	if err != nil {
		return nil, fmt.Errorf("初始化 Casbin enforcer 失败: %w", err)
	}

	// NewSyncedEnforcer 已通过普通 GORM adapter 加载完整策略。
	return enforcer, nil
}
