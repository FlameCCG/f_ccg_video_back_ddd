// Package social 编排关注、取关及关系列表查询。
//
// focus_service.go 负责关系写侧与粉丝指标更新，relation_service.go 负责关系与列表读取。
// 列表能否查看由 domain/social 的隐私规则判断，用户存在性和关注事实由端口取得。
// 关注写入后更新粉丝指标，用户搜索文档由 River 异步派生，阅读错误分支时要区分这些完成点。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package social
