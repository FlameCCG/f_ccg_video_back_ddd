package rbac

// DefaultMenuPolicy 裁定初始化时普通用户与大会员的默认菜单资格。
// 保留历史标题名单，不随路由展示数据的调整扩大授权范围。
type DefaultMenuPolicy struct{}

func (DefaultMenuPolicy) CommonRoleAllowed(title string) bool {
	switch title {
	case "运营总览", "数据看板":
		return true
	default:
		return false
	}
}

func (DefaultMenuPolicy) VIPRoleAllowed(title string) bool {
	switch title {
	case "运营总览", "内容管理", "社区治理", "营销推广", "用户管理", "系统权限",
		"数据看板", "稿件审核", "视频列表", "视频回收站", "分区配置", "举报处理",
		"评论管控", "弹幕管控", "动态管理", "轮播图管理", "全站通知", "封禁管理",
		"封禁记录", "角色管理", "菜单权限", "接口资源":
		return true
	default:
		return false
	}
}
