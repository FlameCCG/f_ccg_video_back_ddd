package po

// VideoLike 用户点赞记录.
type VideoLike struct {
	Model
	VideoID uint `gorm:"not null;index:idx_video_user,unique;index:idx_user_video,priority:2;comment:视频ID" json:"videoId"`
	UserID  uint `gorm:"not null;index:idx_video_user,unique;index:idx_user_video,priority:1;comment:用户ID" json:"userId"`
}

// TableName 方法.
func (VideoLike) TableName() string {
	return "video_likes"
}
