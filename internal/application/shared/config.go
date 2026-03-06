// Package shared 定义多个应用上下文复用的配置访问端口。
//
// ConfigReader/ConfigStore 描述“读取或发布配置”的能力；配置树的数据形状位于 internal/config。
// Viper、文件监听和原子快照属于 infrastructure/config，应用服务不直接导入这些实现。
// 一次业务判断宜保存 Get() 的结果再读取字段；多次 Get() 可能跨越热更新，不能假定总是同一版本。
// arch_deps_test.go 用源码导入检查守护分层与上下文边界，修改依赖前应先阅读这些约束。
package shared

import "f_ccg_video_back_ddd/internal/config"

// ConfigReader 是应用层读取运行期配置的端口。
//
// 配置树支持热更新（文件监听与管理端写回都会整体替换快照），因此用例内
// 每次需要配置时应重新 Get 取本地变量再读字段，避免同一次调用跨版本撕裂：
//
//	cfg := r.Get()
//	if cfg == nil || cfg.Site == nil { ... }
//	use cfg.Site.Login ...
//
// 实现方：*infrastructure/config.Provider（结构化满足，由 bootstrap 注入）。
// 应用层不感知 Provider/viper 等具体机制（见 docs/architecture.md 第 6.2 节）。
type ConfigReader interface {
	// Get 返回当前配置快照；配置尚未加载时可能为 nil。
	Get() *config.Config
}

// ConfigStore 在 ConfigReader 之上增加原子发布新配置树的能力，
// 供管理端配置读写用例（contentops）使用。
//
// 发布前应浅拷贝顶层（config.Clone）、只替换目标配置段，避免与
// 热更新或并发读产生 data race；已发布的配置树视为只读。
type ConfigStore interface {
	ConfigReader
	// Set 原子发布新的配置树。
	Set(c *config.Config)
}
