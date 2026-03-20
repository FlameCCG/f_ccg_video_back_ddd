// Package search 编排关键词搜索、联想、热词、提及和向量索引。
//
// search_service.go 编排搜索与 SQL 回退，vector_service.go/vector_rebuild_service.go 负责向量读写和重建。
// 关键词索引主要由 River 从 binlog 派生，向量索引由应用侧维护；两条更新链路不能视为同一事务。
// 查询 DTO 与写侧实体分开，搜索结果的可见性和回退边界应结合对应端口实现阅读。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package search
