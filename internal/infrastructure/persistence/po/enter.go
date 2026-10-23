// Package po 定义 MySQL/GORM 的持久化对象（Persistence Object）。
//
// 结构体字段、gorm tag、TableName、关联和 hook 描述数据库形状，不等同于领域模型或公开 API DTO。
// 公共 Model 的 ID/时间戳、唯一索引和关联写法都会影响迁移与并发行为，变更前要检查已有数据。
// 各上下文仓储通过 converter 显式转换写模型，读侧也可直接投影为 application DTO。
// videos 中的 is_scheduled、publish_at、approved_at 分别表达定时意图、计划时刻和审核事实。
// 不要把 PO 直接搬入 domain，或为了页面返回字段让领域层依赖 GORM。
package po

import "time"

// Model 提供各 SQL 模型共享的主键和创建/更新时间；不包含 gorm.DeletedAt，嵌入它本身不会启用软删除。
type Model struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}
