// Package rbac 表达角色、菜单、权限覆盖与角色继承规则。
//
// inherit.go 检查继承关系，permission.go 计算授权覆盖，menu_icon.go/menu_suggestion.go 校验不可信展示输入。
// repository.go 定义仓储契约，PathMatcher 把路径匹配能力抽象为端口；本包不创建 Casbin enforcer。
// 菜单是否展示与 API 是否允许调用是不同判断，不能仅凭前端菜单树推导服务端权限。
//
// 本包保持纯 Go 规则与端口，不依赖 HTTP、数据库、缓存、消息框架或配置加载器。
// 领域函数尽量显式接收事实与时间，让规则可以脱离运行环境阅读和验证。
package rbac

import (
	"strings"
	"time"
)

// Menu 菜单实体。
//
// 字段与 json tag 与原 models/mysqlEntity.Menu 对外序列化部分保持一致，保证接口响应 JSON 1:1；
// 持久化细节（gorm tag、多对多关联）留在 infrastructure 的 PO。
// 原实体的反向关联 Roles 从未被任何接口预加载（始终因 omitempty 省略），故领域实体不予保留。
type Menu struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
	Title     string    `json:"title"`
	TitleEn   string    `json:"titleEn"`
	TitleJa   string    `json:"titleJa"`
	Icon      string    `json:"icon"`
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Component string    `json:"component"`
	KeepAlive bool      `json:"keepAlive"`
	ParentID  *uint     `json:"parentId"`
	SortOrder uint      `json:"sortOrder"`
	Children  []*Menu   `json:"children,omitempty"`
}

// BuildMenuTree 构建菜单树（原 pkg/recursion.BuildMenuTree）。
//
// 入参为空时返回 nil，与原实现一致（接口响应中体现为 data:null）。
func BuildMenuTree(menus []*Menu) []*Menu {
	menuMap := make(map[uint]*Menu)

	var roots []*Menu

	for i := range menus {
		menuMap[menus[i].ID] = menus[i]
	}

	for i := range menus {
		menu := menus[i]
		if menu.ParentID == nil || *menu.ParentID == 0 {
			roots = append(roots, menu)
			continue
		}

		if parent, ok := menuMap[*menu.ParentID]; ok {
			parent.Children = append(parent.Children, menu)
			continue
		}
		// 兜底：父菜单不存在时也视为根节点，避免丢失
		roots = append(roots, menu)
	}

	return roots
}

// MenuFieldUpdate 菜单的部分字段更新指令。
//
// 指针字段为 nil 表示本次不更新该列；与原实现「按需拼装 updates map」的语义一致。
type MenuFieldUpdate struct {
	Title     *string
	TitleEn   *string
	TitleJa   *string
	Icon      *string
	Path      *string
	Name      *string
	Component *string
	KeepAlive *bool
	// ParentID 非 nil 时把 parent_id 更新为该值（允许为 0）。
	ParentID *uint
	// ClearParent 为 true 时把 parent_id 置为 NULL；优先级高于 ParentID。
	ClearParent bool
	SortOrder   *uint
}

// IsEmpty 报告本次更新指令是否不含任何待更新字段。
func (u MenuFieldUpdate) IsEmpty() bool {
	return u.Title == nil && u.TitleEn == nil && u.TitleJa == nil && u.Icon == nil &&
		u.Path == nil && u.Name == nil && u.Component == nil && u.KeepAlive == nil &&
		u.ParentID == nil && !u.ClearParent && u.SortOrder == nil
}

// NewMenu 依据创建参数构造菜单实体（字符串字段已由调用方规范化）。
func NewMenu(title, titleEn, titleJa, icon, path, name, component string, keepAlive bool, parentID *uint, sortOrder uint) *Menu {
	return &Menu{
		Title:     title,
		TitleEn:   titleEn,
		TitleJa:   titleJa,
		Icon:      icon,
		Path:      path,
		Name:      name,
		Component: component,
		KeepAlive: keepAlive,
		ParentID:  parentID,
		SortOrder: sortOrder,
	}
}

// TrimOptional 去除可选字符串首尾空白，并报告「原值非空但去空白后为空」这一非法输入。
//
// 对应原 api/rbac_api/create_menu.go 中「req.X != "" && trim(req.X) == ""」的一组校验。
func TrimOptional(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)

	return trimmed, raw == "" || trimmed != ""
}
