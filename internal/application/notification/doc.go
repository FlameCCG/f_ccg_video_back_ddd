// Package notification 编排通知列表、已读状态与角标计数。
//
// DeliveryService 写入通知，InboxService 维护收件箱，AdminService 管理系统通知；CountService 组合私信未读和角标缓存。
// 通知实体不承载用户资料查询；资料和跨上下文计数通过消费方端口取得，写入后再处理角标缓存失效。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package notification
