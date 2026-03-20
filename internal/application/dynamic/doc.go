// Package dynamic 编排动态发布、关注流、点赞、置顶与已读。
//
// dynamic_service.go 编排写侧，feed_service.go/follow_service.go/read_service.go 组织读侧和已读状态。
// 置顶规则、作品类型和已读判定由领域负责；关联视频与清理评论经本地端口协作，避免直接依赖其他上下文服务。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package dynamic
