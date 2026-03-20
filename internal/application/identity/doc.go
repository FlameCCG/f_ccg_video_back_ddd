// Package identity 编排账号、登录、第三方认证、封禁、经验与硬币。
//
// 从 login_service.go、registration_service.go 进入主流程；对应的 *_ports.go 描述令牌、验证码与第三方能力。
// 领域实体只表达身份规则，密码散列、JWT、OAuth HTTP、缓存与数据库由适配器实现。
// 账号改名等操作还影响视频作者和搜索读模型，协作端口由 bootstrap/adapters 接到提供方。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package identity
