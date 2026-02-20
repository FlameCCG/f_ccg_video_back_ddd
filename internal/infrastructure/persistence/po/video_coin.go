package po

// VideoCoin 用户投币记录.
type VideoCoin struct {
	Model
	VideoID uint `gorm:"not null;index:idx_video_user,unique;comment:视频ID" json:"videoId"`
	UserID  uint `gorm:"not null;index:idx_video_user,unique;comment:用户ID" json:"userId"`
	Coins   int  `gorm:"not null;default:1;comment:投币数量(1-2)" json:"coins"`
}

// TableName 方法.
func (VideoCoin) TableName() string {
	return "video_coins"
}
