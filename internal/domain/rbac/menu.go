package rbac

import (
	"strings"
	"time"
)

// Menu 菜单实体，状态不对外暴露，HTTP 响应使用查询 DTO。
//
// gorm tag 和多对多关联由 infrastructure 的 PO 承载；领域实体不持有反向角色关联。
type Menu struct {
	id        uint
	createdAt time.Time
	updatedAt time.Time
	title     string
	titleEn   string
	titleJa   string
	icon      string
	path      string
	name      string
	component string
	keepAlive bool
	parentID  *uint
	sortOrder uint
	children  []*Menu
}

// MenuSnapshot 是菜单的持久化与历史重建快照，字段公开。
type MenuSnapshot struct {
	ID        uint
	CreatedAt time.Time
	UpdatedAt time.Time
	Title     string
	TitleEn   string
	TitleJa   string
	Icon      string
	Path      string
	Name      string
	Component string
	KeepAlive bool
	ParentID  *uint
	SortOrder uint
	Children  []MenuSnapshot
}

// MenuFieldUpdate 菜单的部分字段更新指令。
//
// 指针字段为 nil 表示本次不更新该列。
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

// NewMenu 按既有创建规则规范化并校验后构造菜单。
// 父级是否存在、是否成环以及标题唯一性需要外部事实，不在这里判断。
func NewMenu(title, titleEn, titleJa, icon, path, name, component string, keepAlive bool, parentID *uint, sortOrder uint) (*Menu, error) {
	menu := &Menu{
		title:     strings.TrimSpace(title),
		titleEn:   strings.TrimSpace(titleEn),
		titleJa:   strings.TrimSpace(titleJa),
		icon:      strings.TrimSpace(icon),
		path:      strings.TrimSpace(path),
		name:      strings.TrimSpace(name),
		component: strings.TrimSpace(component),
		keepAlive: keepAlive,
		parentID:  (Menu{}).cloneUint(parentID),
		sortOrder: sortOrder,
	}
	if menu.title == "" {
		return nil, ErrMenuTitleEmpty
	}
	if err := MenuIcon(menu.icon).Validate(); err != nil {
		return nil, err
	}

	return menu, nil
}

// RestoreMenu 从快照恢复历史菜单，不重新执行创建校验。
func RestoreMenu(snapshot MenuSnapshot) *Menu {
	menu := &Menu{
		id:        snapshot.ID,
		createdAt: snapshot.CreatedAt,
		updatedAt: snapshot.UpdatedAt,
		title:     snapshot.Title,
		titleEn:   snapshot.TitleEn,
		titleJa:   snapshot.TitleJa,
		icon:      snapshot.Icon,
		path:      snapshot.Path,
		name:      snapshot.Name,
		component: snapshot.Component,
		keepAlive: snapshot.KeepAlive,
		parentID:  (Menu{}).cloneUint(snapshot.ParentID),
		sortOrder: snapshot.SortOrder,
	}
	if snapshot.Children != nil {
		menu.children = make([]*Menu, len(snapshot.Children))
		for i := range snapshot.Children {
			menu.children[i] = RestoreMenu(snapshot.Children[i])
		}
	}

	return menu
}

// Snapshot 复制当前菜单。修改返回值不会影响实体。
func (m *Menu) Snapshot() MenuSnapshot {
	if m == nil {
		return MenuSnapshot{}
	}

	snapshot := MenuSnapshot{
		ID:        m.id,
		CreatedAt: m.createdAt,
		UpdatedAt: m.updatedAt,
		Title:     m.title,
		TitleEn:   m.titleEn,
		TitleJa:   m.titleJa,
		Icon:      m.icon,
		Path:      m.path,
		Name:      m.name,
		Component: m.component,
		KeepAlive: m.keepAlive,
		ParentID:  (Menu{}).cloneUint(m.parentID),
		SortOrder: m.sortOrder,
	}
	if m.children != nil {
		snapshot.Children = make([]MenuSnapshot, len(m.children))
		for i, child := range m.children {
			if child != nil {
				snapshot.Children[i] = child.Snapshot()
			}
		}
	}

	return snapshot
}

func (m *Menu) ID() uint {
	if m == nil {
		return 0
	}

	return m.id
}
func (m *Menu) CreatedAt() time.Time {
	if m == nil {
		return time.Time{}
	}

	return m.createdAt
}
func (m *Menu) UpdatedAt() time.Time {
	if m == nil {
		return time.Time{}
	}

	return m.updatedAt
}
func (m *Menu) Title() string {
	if m == nil {
		return ""
	}

	return m.title
}
func (m *Menu) TitleEn() string {
	if m == nil {
		return ""
	}

	return m.titleEn
}
func (m *Menu) TitleJa() string {
	if m == nil {
		return ""
	}

	return m.titleJa
}
func (m *Menu) Icon() string {
	if m == nil {
		return ""
	}

	return m.icon
}
func (m *Menu) Path() string {
	if m == nil {
		return ""
	}

	return m.path
}
func (m *Menu) Name() string {
	if m == nil {
		return ""
	}

	return m.name
}
func (m *Menu) Component() string {
	if m == nil {
		return ""
	}

	return m.component
}
func (m *Menu) KeepAlive() bool {
	if m == nil {
		return false
	}

	return m.keepAlive
}

// ParentID 返回父级副本。调用方改指针不会影响菜单。
func (m *Menu) ParentID() *uint {
	if m == nil {
		return nil
	}

	return (Menu{}).cloneUint(m.parentID)
}
func (m *Menu) SortOrder() uint {
	if m == nil {
		return 0
	}

	return m.sortOrder
}

// Children 返回子菜单副本。
func (m *Menu) Children() []*Menu {
	if m == nil || m.children == nil {
		return nil
	}

	cloned := make([]*Menu, len(m.children))
	for i, child := range m.children {
		cloned[i] = child.clone()
	}

	return cloned
}

func (m *Menu) clone() *Menu {
	if m == nil {
		return nil
	}

	copied := *m
	copied.parentID = (Menu{}).cloneUint(m.parentID)
	if m.children != nil {
		copied.children = make([]*Menu, len(m.children))
		for i, child := range m.children {
			copied.children[i] = child.clone()
		}
	}

	return &copied
}

func (Menu) cloneUint(value *uint) *uint {
	if value == nil {
		return nil
	}

	copied := *value

	return &copied
}

// ApplySortOrder 记下同层插入计划给出的排序位置，供随后的插入使用。
func (m *Menu) ApplySortOrder(sortOrder uint) {
	if m == nil {
		return
	}

	m.sortOrder = sortOrder
}

// EnsureTitleAvailable 同标题菜单已存在时拒绝保存。
func (m *Menu) EnsureTitleAvailable(count int64) error {
	if m == nil {
		return ErrMenuNotFound
	}
	if count > 0 {
		return ErrMenuExists
	}

	return nil
}

// EnsureParent 根据调用方逐级取回的父菜单判断父级是否存在、是否指向自己或成环。
// 失败时不修改菜单。lookup 的错误原样返回，由用例决定如何包装基础设施错误。
func (m *Menu) EnsureParent(parentID *uint, lookup func(uint) (*Menu, error)) error {
	next := (MenuOrder{}).NormalizeParent(parentID)
	if next == nil {
		return nil
	}

	self := uint(0)
	if m != nil {
		self = m.id
	}

	visited := make(map[uint]struct{})
	current := *next
	for current > 0 {
		if self > 0 && current == self {
			return ErrMenuParentCycle
		}
		if _, ok := visited[current]; ok {
			return ErrMenuParentCycle
		}
		visited[current] = struct{}{}
		if lookup == nil {
			return ErrParentMenuNotFound
		}

		parent, err := lookup(current)
		if err != nil {
			return err
		}
		if parent == nil {
			return ErrParentMenuNotFound
		}

		normalized := (MenuOrder{}).NormalizeParent(parent.ParentID())
		if normalized == nil {
			return nil
		}

		current = *normalized
	}

	return nil
}
