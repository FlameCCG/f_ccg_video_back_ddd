// Package site 定义站点级登录、注册、存储、审核和投稿开关的数据结构。
//
// 各子结构体使用指针，字段缺失与显式关闭可能走不同的默认逻辑；读取方需要检查 nil。
// 这里只描述可配置项，不决定某次请求的用户资格；实际授权与校验由中间件和应用用例执行。
// 修改配置键时同时检查 config.example.yaml、读取逻辑和管理端 JSON 契约。
package site

// Site 聚合站点可调整的功能策略；读取方先检查子配置是否存在，再应用各自默认规则。
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
