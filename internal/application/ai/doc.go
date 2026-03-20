// Package ai 编排AI 请求编排、站内提示词增强和菜单建议生成。
//
// 按 conversation/image/video/asset 的服务、类型与端口阅读；ai_service.go 保留构造和配置快照；conversation.go 处理会话裁剪，prompt_service.go 组织站内作品上下文。
// 模型输出仍是外部输入：菜单候选要经过领域校验，流式响应一旦开始就不能再按普通 JSON 错误响应处理。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package ai
