package po

// CommentLike 评论点赞记录.
type CommentLike struct {
	Model
	CommentID uint    `gorm:"not null;index:idx_comment_user,unique;comment:评论ID" json:"commentId"`
	UserID    uint    `gorm:"not null;index:idx_comment_user,unique;comment:用户ID" json:"userId"`
	User      User    `gorm:"foreignKey:UserID" json:"user"`
	Comment   Comment `gorm:"foreignKey:CommentID" json:"comment"`
}

// TableName 显式绑定表名 comment_likes，保持迁移与仓储 SQL 使用同一张表。
func (CommentLike) TableName() string {
	return "comment_likes"
}
