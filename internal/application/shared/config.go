// Package shared 声明跨限界上下文复用的应用层端口。
//
// 本包只放端口契约（接口）与其直接关联的值类型，不放实现；
// 实现一律位于 infrastructure，由 bootstrap 装配（依赖倒置，与 domain/shared/tx 同一模式）。
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
// 应用层不感知 Provider/viper 等具体机制（见 docs/decisions.md D-020）。
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
