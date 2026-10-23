package po

// Danmu 按作品、分P和播放时间偏移保存弹幕；TimeOffset 是片内秒数，CreatedAt 才是发送时间。
type Danmu struct {
	Model
	UserID      uint   `gorm:"not null;comment:用户ID" json:"userId"`
	VideoID     uint   `gorm:"not null;index:idx_video_time;index:idx_video_part_time,priority:1;comment:视频ID" json:"videoId"`
	VideoPartID uint   `gorm:"not null;default:0;index:idx_video_part_time,priority:2;comment:分P ID" json:"videoPartId"`
	Content     string `gorm:"not null;type:text;comment:弹幕内容" json:"content"`
	TimeOffset  int    `gorm:"not null;default:0;index:idx_video_time;index:idx_video_part_time,priority:3;comment:弹幕时间偏移(秒)" json:"timeOffset"`
	Color       string `gorm:"default:#FFFFFF;size:20;comment:弹幕颜色" json:"color"`
	FontSize    int8   `gorm:"default:25;comment:字体大小" json:"fontSize"`
	Position    int8   `gorm:"default:0;comment:位置: 0-滚动 1-顶部 2-底部" json:"position"`
	LikeCount   int64  `gorm:"default:0;comment:点赞数" json:"likeCount"`
	Video       Video  `gorm:"foreignKey:VideoID" json:"video,omitempty"`
	User        User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName 显式绑定表名 danmu，保持迁移与仓储 SQL 使用同一张表。
func (Danmu) TableName() string {
	return "danmu"
}
