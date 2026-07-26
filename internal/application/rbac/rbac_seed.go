package rbac

// 默认菜单/角色种子数据（原 service/rbac_service/init_service.go 内联字面量，逐字搬迁）。
//
// 以下包级变量只做只读查表，任何调用都不会修改它们：
// buildMenuSeeds 会先复制一份再回填图标，避免出现包级可变状态。
const (
	menuComponentLayout     = "Layout"
	menuTitleContent        = "内容管理"
	menuTitleCommunity      = "社区治理"
	menuTitleMarketing      = "营销推广"
	menuTitleUsers          = "用户管理"
	menuTitleSystem         = "系统权限"
	menuSiteConfigComponent = "views/system/site-config/index"
	menuTitleSiteConfig     = "系统配置"
)

// menuSeed 默认菜单的一条种子定义。
type menuSeed struct {
	Title       string
	TitleEn     string
	TitleJa     string
	Icon        string
	Path        string
	Name        string
	Component   string
	KeepAlive   bool
	SortOrder   uint
	ParentTitle string
}

// defaultMenuIcons 菜单路由名 → 内联 SVG 图标（只读）。
var defaultMenuIcons = map[string]string{
	"default":              `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/></svg>`,
	"Overview":             `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M12 12l4-4"/><path d="M7 12a5 5 0 0 1 10 0"/></svg>`,
	"Content":              `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="5" width="18" height="14" rx="2"/><path d="M7 5v14M17 5v14M3 9h4M3 15h4M17 9h4M17 15h4"/></svg>`,
	"Community":            `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="8" cy="8" r="3"/><circle cx="16" cy="8" r="3"/><path d="M2 20c0-3 3-5 6-5"/><path d="M22 20c0-3-3-5-6-5"/></svg>`,
	"Marketing":            `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 11l13-5v12L3 13z"/><path d="M16 6l5-2v16l-5-2"/><path d="M7 16v3a2 2 0 0 0 2 2h1"/></svg>`,
	"Users":                `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="4"/><path d="M4 20c2-4 6-6 8-6s6 2 8 6"/></svg>`,
	"System":               `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3l8 4v5c0 5-3.5 9-8 9s-8-4-8-9V7l8-4z"/></svg>`,
	"SiteConfig":           `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6h10"/><circle cx="17" cy="6" r="2"/><path d="M4 12h6"/><circle cx="13" cy="12" r="2"/><path d="M4 18h14"/><circle cx="7" cy="18" r="2"/></svg>`,
	"DataDashboard":        `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19h16"/><rect x="6" y="10" width="2" height="7"/><rect x="11" y="7" width="2" height="10"/><rect x="16" y="4" width="2" height="13"/></svg>`,
	"SubmissionReview":     `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M8 12l3 3 5-6"/></svg>`,
	"VideoList":            `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 6h10"/><path d="M4 12h10"/><path d="M4 18h10"/><polygon points="16,9 22,12 16,15"/></svg>`,
	"VideoRecycle":         `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="3 6 5 6 21 6"/><path d="M8 6V4h8v2"/><rect x="6" y="6" width="12" height="14" rx="2"/></svg>`,
	"CategoryConfig":       `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="4" width="6" height="6"/><rect x="14" y="4" width="6" height="6"/><rect x="4" y="14" width="6" height="6"/><rect x="14" y="14" width="6" height="6"/></svg>`,
	"ReportHandling":       `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3l10 18H2L12 3z"/><path d="M12 9v4"/><path d="M12 17h.01"/></svg>`,
	"CommentModeration":    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>`,
	"DanmakuModeration":    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3a9 9 0 0 0-9 9c0 2.4 1 4.6 2.7 6.2L5 21l3.1-1.1A9 9 0 1 0 12 3z"/></svg>`,
	"ActivityManagement":   `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2L3 14h7l-1 8 10-12h-7z"/></svg>`,
	"BannerManagement":     `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="5" width="18" height="14" rx="2"/><circle cx="8" cy="10" r="2"/><path d="M21 16l-5-5-4 4-2-2-5 5"/></svg>`,
	"SiteNotifications":    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 7h18s-3 0-3-7"/><path d="M13.5 21a1.5 1.5 0 0 1-3 0"/></svg>`,
	"UserList":             `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="4"/><path d="M4 20c2-4 6-6 8-6s6 2 8 6"/></svg>`,
	"BanManagement":        `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M6 6l12 12"/></svg>`,
	"BanRecords":           `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 1 0 3-6"/><polyline points="3 5 3 12 10 12"/><path d="M12 7v5l3 3"/></svg>`,
	"RoleManagement":       `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="9" cy="10" r="2"/><path d="M7 16c1-2 4-2 5 0"/><path d="M14 8h4"/><path d="M14 12h4"/></svg>`,
	"MenuPermissions":      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="4" y1="6" x2="20" y2="6"/><line x1="4" y1="12" x2="20" y2="12"/><line x1="4" y1="18" x2="20" y2="18"/></svg>`,
	"ApiResources":         `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="8 6 3 12 8 18"/><polyline points="16 6 21 12 16 18"/></svg>`,
	"SiteConfigSite":       `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 10l9-7 9 7v9a2 2 0 0 1-2 2h-4v-6H9v6H5a2 2 0 0 1-2-2z"/></svg>`,
	"SiteConfigLogger":     `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="3" width="16" height="18" rx="2"/><line x1="8" y1="9" x2="16" y2="9"/><line x1="8" y1="13" x2="16" y2="13"/><line x1="8" y1="17" x2="12" y2="17"/></svg>`,
	"SiteConfigEmail":      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 4h16a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2z"/><polyline points="22 6 12 13 2 6"/></svg>`,
	"SiteConfigTranscode":  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><path d="M20.5 13a8 8 0 1 1 2.1-7"/><polyline points="1 20 1 14 7 14"/><path d="M3.5 11a8 8 0 1 1-2.1 7"/></svg>`,
	"SiteConfigThirdLogin": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="7" cy="12" r="3"/><path d="M10 12h11l-2 2 2 2-2 2"/></svg>`,
	"SiteConfigJwt":        `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="5" y="10" width="14" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/></svg>`,
}

// defaultMenuSeeds 默认菜单种子表（只读；顺序即创建顺序，父菜单先于子菜单）。
var defaultMenuSeeds = []menuSeed{
	{Title: "仪表盘", TitleEn: "Dashboard", TitleJa: "ダッシュボード", Icon: "", Path: "/overview", Name: "Overview", Component: menuComponentLayout, SortOrder: 1},
	{Title: menuTitleContent, TitleEn: "Content Management", TitleJa: "コンテンツ管理", Icon: "", Path: "/content", Name: "Content", Component: menuComponentLayout, SortOrder: 2},
	{Title: menuTitleCommunity, TitleEn: "Community Moderation", TitleJa: "コミュニティ管理", Icon: "", Path: "/community", Name: "Community", Component: menuComponentLayout, SortOrder: 3},
	{Title: menuTitleMarketing, TitleEn: "Marketing & Promotion", TitleJa: "マーケティング", Icon: "", Path: "/marketing", Name: "Marketing", Component: menuComponentLayout, SortOrder: 4},
	{Title: menuTitleUsers, TitleEn: "User Management", TitleJa: "ユーザー管理", Icon: "", Path: "/users", Name: "Users", Component: menuComponentLayout, SortOrder: 5},
	{Title: menuTitleSystem, TitleEn: "System Permissions", TitleJa: "システム権限", Icon: "", Path: "/system", Name: "System", Component: menuComponentLayout, SortOrder: 6},
	{Title: menuTitleSiteConfig, TitleEn: "System Configuration", TitleJa: "システム設定", Icon: "", Path: "/system-config", Name: "SiteConfig", Component: menuSiteConfigComponent, SortOrder: 7},
	{Title: "稿件审核", TitleEn: "Submission Review", TitleJa: "投稿審査", Icon: "", Path: "review", Name: "SubmissionReview", Component: "views/content/review/index", SortOrder: 1, ParentTitle: menuTitleContent},
	{Title: "视频列表", TitleEn: "Video List", TitleJa: "動画一覧", Icon: "", Path: "videos", Name: "VideoList", Component: "views/content/videos/index", SortOrder: 2, ParentTitle: menuTitleContent},
	{Title: "视频回收站", TitleEn: "Video Recycle Bin", TitleJa: "動画ごみ箱", Icon: "", Path: "recycle", Name: "VideoRecycle", Component: "views/content/recycle/index", SortOrder: 3, ParentTitle: menuTitleContent},
	{Title: "分区配置", TitleEn: "Category Configuration", TitleJa: "カテゴリ設定", Icon: "", Path: "categories", Name: "CategoryConfig", Component: "views/content/categories/index", SortOrder: 4, ParentTitle: menuTitleContent},
	{Title: "举报处理", TitleEn: "Report Handling", TitleJa: "通報対応", Icon: "", Path: "reports", Name: "ReportHandling", Component: "views/community/reports/index", SortOrder: 1, ParentTitle: menuTitleCommunity},
	{Title: "评论管控", TitleEn: "Comment Moderation", TitleJa: "コメント管理", Icon: "", Path: "comments", Name: "CommentModeration", Component: "views/community/comments/index", SortOrder: 2, ParentTitle: menuTitleCommunity},
	{Title: "弹幕管控", TitleEn: "Danmaku Moderation", TitleJa: "弾幕管理", Icon: "", Path: "danmaku", Name: "DanmakuModeration", Component: "views/community/danmaku/index", SortOrder: 3, ParentTitle: menuTitleCommunity},
	{Title: "动态管理", TitleEn: "Activity Management", TitleJa: "アクティビティ管理", Icon: "", Path: "activities", Name: "ActivityManagement", Component: "views/community/activities/index", SortOrder: 4, ParentTitle: menuTitleCommunity},
	{Title: "轮播图管理", TitleEn: "Banner Management", TitleJa: "バナー管理", Icon: "", Path: "banners", Name: "BannerManagement", Component: "views/marketing/banners/index", SortOrder: 1, ParentTitle: menuTitleMarketing},
	{Title: "全站通知", TitleEn: "Site-wide Notifications", TitleJa: "全体通知", Icon: "", Path: "notifications", Name: "SiteNotifications", Component: "views/marketing/notifications/index", SortOrder: 2, ParentTitle: menuTitleMarketing},
	{Title: "用户列表", TitleEn: "User List", TitleJa: "ユーザー一覧", Icon: "", Path: "list", Name: "UserList", Component: "views/users/list/index", SortOrder: 1, ParentTitle: menuTitleUsers},
	{Title: "封禁管理", TitleEn: "Ban Management", TitleJa: "凍結管理", Icon: "", Path: "bans", Name: "BanManagement", Component: "views/users/bans/index", SortOrder: 2, ParentTitle: menuTitleUsers},
	{Title: "封禁记录", TitleEn: "Ban Records", TitleJa: "凍結記録", Icon: "", Path: "ban-records", Name: "BanRecords", Component: "views/users/ban-records/index", SortOrder: 3, ParentTitle: menuTitleUsers},
	{Title: "角色管理", TitleEn: "Role Management", TitleJa: "ロール管理", Icon: "", Path: "roles", Name: "RoleManagement", Component: "views/system/roles/index", SortOrder: 1, ParentTitle: menuTitleSystem},
	{Title: "菜单权限", TitleEn: "Menu Permissions", TitleJa: "メニュー権限", Icon: "", Path: "menus", Name: "MenuPermissions", Component: "views/system/menus/index", SortOrder: 2, ParentTitle: menuTitleSystem},
	{Title: "接口资源", TitleEn: "API Resources", TitleJa: "APIリソース", Icon: "", Path: "apis", Name: "ApiResources", Component: "views/system/apis/index", SortOrder: 3, ParentTitle: menuTitleSystem},
	{Title: "基础配置", TitleEn: "Basic Settings", TitleJa: "基本設定", Icon: "", Path: "site", Name: "SiteConfigSite", Component: menuSiteConfigComponent, SortOrder: 1, ParentTitle: menuTitleSiteConfig},
	{Title: "日志配置", TitleEn: "Logger Settings", TitleJa: "ログ設定", Icon: "", Path: "logger", Name: "SiteConfigLogger", Component: menuSiteConfigComponent, SortOrder: 2, ParentTitle: menuTitleSiteConfig},
	{Title: "邮件配置", TitleEn: "Email Settings", TitleJa: "メール設定", Icon: "", Path: "email", Name: "SiteConfigEmail", Component: menuSiteConfigComponent, SortOrder: 3, ParentTitle: menuTitleSiteConfig},
	{Title: "转码配置", TitleEn: "Transcode Settings", TitleJa: "トランスコード設定", Icon: "", Path: "transcode", Name: "SiteConfigTranscode", Component: menuSiteConfigComponent, SortOrder: 4, ParentTitle: menuTitleSiteConfig},
	{Title: "第三方登录", TitleEn: "Login Settings", TitleJa: "ログイン設定", Icon: "", Path: "thirdLogin", Name: "SiteConfigThirdLogin", Component: menuSiteConfigComponent, SortOrder: 5, ParentTitle: menuTitleSiteConfig},
	{Title: "JWT配置", TitleEn: "JWT Settings", TitleJa: "JWT設定", Icon: "", Path: "jwt", Name: "SiteConfigJwt", Component: menuSiteConfigComponent, SortOrder: 6, ParentTitle: menuTitleSiteConfig},
}

// commonRoleMenuTitles 普通用户默认可见的菜单标题（只读）。
var commonRoleMenuTitles = map[string]struct{}{
	"运营总览": {},
	"数据看板": {},
}

// vipRoleMenuTitles 大会员默认可见的菜单标题（只读）。
var vipRoleMenuTitles = map[string]struct{}{
	"运营总览":             {},
	menuTitleContent:   {},
	menuTitleCommunity: {},
	menuTitleMarketing: {},
	menuTitleUsers:     {},
	menuTitleSystem:    {},
	"数据看板":             {},
	"稿件审核":             {},
	"视频列表":             {},
	"视频回收站":            {},
	"分区配置":             {},
	"举报处理":             {},
	"评论管控":             {},
	"弹幕管控":             {},
	"动态管理":             {},
	"轮播图管理":            {},
	"全站通知":             {},
	"封禁管理":             {},
	"封禁记录":             {},
	"角色管理":             {},
	"菜单权限":             {},
	"接口资源":             {},
}

// buildMenuSeeds 复制一份种子表并回填图标，图标缺失时回落到默认图标。
func buildMenuSeeds() []menuSeed {
	seeds := make([]menuSeed, len(defaultMenuSeeds))
	copy(seeds, defaultMenuSeeds)

	for i := range seeds {
		if icon, ok := defaultMenuIcons[seeds[i].Name]; ok {
			seeds[i].Icon = icon
			continue
		}

		seeds[i].Icon = defaultMenuIcons["default"]
	}

	return seeds
}
