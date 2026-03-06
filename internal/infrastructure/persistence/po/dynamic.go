package po

import "time"

// Dynamic 用户动态（图文）.
type Dynamic struct {
	Model
	UserID    uint       `gorm:"not null;index;comment:发布用户ID" json:"userId"`
	Content   string     `gorm:"type:text;not null;comment:动态内容" json:"content"`
	ImageURL  string     `gorm:"size:500;comment:图片URL" json:"imageUrl"`
	LikeCount int64      `gorm:"default:0;comment:点赞数" json:"likeCount"`
	IsPinned  bool       `gorm:"default:false;index;comment:是否置顶" json:"isPinned"`
	PinnedAt  *time.Time `gorm:"comment:置顶时间" json:"pinnedAt,omitempty"`
	User      User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName 显式绑定表名 dynamics，保持迁移与仓储 SQL 使用同一张表。
func (Dynamic) TableName() string {
	return "dynamics"
}
