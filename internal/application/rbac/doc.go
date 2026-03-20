// Package rbac 编排角色、菜单、API 权限与继承关系管理。
//
// 先读 permission_ports.go 的授权引擎端口，再读 permission_service.go、inherit_service.go 和 role_graph.go。
// 领域表达继承和权限覆盖规则，Casbin 策略读写由适配器实现；菜单是展示结构，API 鉴权仍需服务端策略。
// resource_sync_service.go 以实际路由清单同步 API 资源；menu_generation.go 对 AI 候选再次校验后才进入写入流程。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package rbac
