package config

import "f_ccg_video_back_ddd/internal/config/thirdlogin"

// ThirdLogin 类型.
type ThirdLogin struct {
	QQ      *third_login.QQ      `mapstructure:"qq" json:"qq"`
	Google  *third_login.Google  `mapstructure:"google" json:"google"`
	GitHub  *third_login.GitHub  `mapstructure:"github" json:"github"`
	LinuxDo *third_login.LinuxDo `mapstructure:"linuxdo" json:"linuxdo"`
	X       *third_login.X       `mapstructure:"x" json:"x"`
}
