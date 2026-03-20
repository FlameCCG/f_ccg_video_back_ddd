// Package contentops 编排站点配置、轮播图与站点统计。
//
// site_config_service.go 经 shared.ConfigStore 修改配置，banner_service.go 组合规则、仓储与缓存。
// 配置发布到内存与写回 YAML 不是原子事务；文件写入失败时应区分当前生效值与重启后会加载的值。
//
// 本层接收 context.Context 与命令/查询参数，不接收 gin.Context，也不选择 GORM、Redis 或 Kafka 实现。
// provider.go 只汇总本上下文构造器；具体实现和跨上下文端口在 bootstrap 绑定。
package contentops
