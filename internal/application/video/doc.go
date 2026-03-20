// Package video 编排投稿、改稿、播放详情、互动、弹幕与管理。
//
// 建议按 UploadService → PublishService → domain/video → persistence/video 的顺序追一次投稿。
// PublishService/UpdateService 编排用例，domain 维护状态与改稿规则；各业务能力的 *_ports.go 定义查询、缓存及跨上下文能力。
// 写侧通过领域仓储持久化，读侧直接组装 对应的 *_types.go 的页面模型，这是共用存储的轻量 CQRS。
// 定时计划与视频状态同库保存；转码投递、向量更新和缓存维护有各自完成点，不属于同一个分布式事务。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package video
