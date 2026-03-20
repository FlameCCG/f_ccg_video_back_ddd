// Package comment 编排评论创建、回复、点赞与管理。
//
// 从 create_service.go、delete_service.go、pin_service.go、query_service.go、like_service.go 进入用例；缓存重建和通知由包内组件协作。
// 评论持久化、Redis 点赞和通知各有失败边界；不能从一次 HTTP 成功推断所有派生计数都已回写。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package comment
