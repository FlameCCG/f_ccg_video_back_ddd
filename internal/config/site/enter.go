// Package site provides package functionality.
package site

// Site 类型.
type Site struct {
	// DisableUserSubmission 开启后仅管理员可上传、投稿和改稿；缺省为 false。
	DisableUserSubmission bool           `mapstructure:"disableUserSubmission" json:"disableUserSubmission"`
	Login                 *Login         `mapstructure:"login" json:"login"`
	Register              *Register      `mapstructure:"register" json:"register"`
	Storage               *Storage       `mapstructure:"storage" json:"storage"`
	ContentReview         *ContentReview `mapstructure:"contentReview" json:"contentReview"`
	DefaultUserBannerID   uint           `mapstructure:"defaultUserBannerID" json:"defaultUserBannerID"`
	DefaultUserBannerIDs  []uint         `mapstructure:"defaultUserBannerIDs" json:"defaultUserBannerIDs"`
}
