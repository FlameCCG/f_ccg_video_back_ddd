package config

import third_login "f_ccg_video_back_ddd/internal/config/thirdlogin"

// ThirdLogin 聚合各第三方登录客户端配置；指针缺省需由读取方与站点启用开关共同判断。
type ThirdLogin struct {
	QQ      *third_login.QQ      `mapstructure:"qq" json:"qq"`
	Google  *third_login.Google  `mapstructure:"google" json:"google"`
	GitHub  *third_login.GitHub  `mapstructure:"github" json:"github"`
	LinuxDo *third_login.LinuxDo `mapstructure:"linuxdo" json:"linuxdo"`
	X       *third_login.X       `mapstructure:"x" json:"x"`
}
