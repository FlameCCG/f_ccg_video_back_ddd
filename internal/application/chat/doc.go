// Package chat 编排私信发送、会话可见性与已读推进。
//
// chat_service.go 编排参与者、关注关系、发送限制和消息存储；规则由 domain/chat 表达。
// 消息写入与事件广播是两个完成点。WebSocket/Redis 通知用于实时展示，读取历史仍需经过查询端口。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package chat
