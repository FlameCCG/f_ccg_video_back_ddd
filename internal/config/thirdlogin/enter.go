// Package third_login provides package functionality.
package third_login

// QQ 类型.
type QQ struct {
	AppID    string `mapstructure:"appID" json:"appID"`
	AppKey   string `mapstructure:"appKey" json:"appKey"`
	Redirect string `mapstructure:"redirect" json:"redirect"`
}

// Google 类型.
type Google struct {
	ClientID     string `mapstructure:"clientID" json:"clientID"`
	ClientSecret string `mapstructure:"clientSecret" json:"clientSecret"`
	Redirect     string `mapstructure:"redirect" json:"redirect"`
}

// GitHub 类型.
type GitHub struct {
	ClientID     string `mapstructure:"clientID" json:"clientID"`
	ClientSecret string `mapstructure:"clientSecret" json:"clientSecret"`
	Redirect     string `mapstructure:"redirect" json:"redirect"`
}

// LinuxDo 类型.
type LinuxDo struct {
	ClientID     string `mapstructure:"clientID" json:"clientID"`
	ClientSecret string `mapstructure:"clientSecret" json:"clientSecret"`
	Redirect     string `mapstructure:"redirect" json:"redirect"`
}

// X 类型.
type X struct {
	ClientID     string `mapstructure:"clientID" json:"clientID"`
	ClientSecret string `mapstructure:"clientSecret" json:"clientSecret"`
	Redirect     string `mapstructure:"redirect" json:"redirect"`
}
