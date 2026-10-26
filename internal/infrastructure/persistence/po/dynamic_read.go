package po

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"time"
)

// DynamicRead 用户对“关注的人最新作品”的已读记录
// 一条记录代表 user_id 对 author_id 的“最新作品”已读到哪个作品（按作品发布时间比对）。
type DynamicRead struct {
	Model

	UserID   uint `gorm:"not null;uniqueIndex:idx_dynamic_read;comment:阅读用户ID" json:"userId"`
	AuthorID uint `gorm:"not null;uniqueIndex:idx_dynamic_read;comment:作者用户ID" json:"authorId"`

	LastReadWorkType enum.WorkType `gorm:"not null;comment:最后已读作品类型" json:"lastReadWorkType"`
	LastReadWorkID   uint          `gorm:"not null;comment:最后已读作品ID" json:"lastReadWorkId"`
	LastReadWorkAt   time.Time     `gorm:"not null;index;comment:最后已读作品发布时间" json:"lastReadWorkAt"`

	Author User `gorm:"foreignKey:AuthorID" json:"author"`
	User   User `gorm:"foreignKey:UserID" json:"user"`
}

// TableName 显式绑定表名 dynamic_reads，保持迁移与仓储 SQL 使用同一张表。
func (DynamicRead) TableName() string {
	return "dynamic_reads"
}
