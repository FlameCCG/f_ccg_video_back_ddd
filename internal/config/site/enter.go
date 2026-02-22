// Package site provides package functionality.
package site

// Site 类型.
type Site struct {
	Login                *Login         `mapstructure:"login" json:"login"`
	Register             *Register      `mapstructure:"register" json:"register"`
	Storage              *Storage       `mapstructure:"storage" json:"storage"`
	ContentReview        *ContentReview `mapstructure:"contentReview" json:"contentReview"`
	DefaultUserBannerID  uint           `mapstructure:"defaultUserBannerID" json:"defaultUserBannerID"`
	DefaultUserBannerIDs []uint         `mapstructure:"defaultUserBannerIDs" json:"defaultUserBannerIDs"`
}
