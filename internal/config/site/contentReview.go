package site

// ContentReview 决定公开投稿是否进入人工审核，应用层将 Enable 作为普通布尔值传给领域状态规则。
type ContentReview struct {
	Enable bool `mapstructure:"enable" json:"enable"`
}
