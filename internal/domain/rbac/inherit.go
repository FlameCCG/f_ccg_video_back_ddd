package rbac

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// InheritKind 继承层级（原 api/rbac_api/role_inherit_helpers.go 的 inheritKind）。
type InheritKind int

const (
	// InheritKindNone 无继承关系.
	InheritKindNone InheritKind = iota
	// InheritKindDirect 直接继承.
	InheritKindDirect
	// InheritKindIndirect 间接继承（经继承链获得）.
	InheritKindIndirect
)

// userSubjectPrefix Casbin 中用户主体的前缀，用于把「用户-角色」分组策略与「角色-角色」区分开。
const userSubjectPrefix = "user:"

// ContainsRoleName 判断角色名列表中是否包含目标（忽略空白）。
func ContainsRoleName(names []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}

	for _, n := range names {
		if strings.TrimSpace(n) == target {
			return true
		}
	}

	return false
}

// FormatAlreadyInheritedMsg 已继承时的提示文案。
func FormatAlreadyInheritedMsg(targetRole string, kind InheritKind, topParent string) string {
	switch kind {
	case InheritKindDirect:
		return fmt.Sprintf("已直接继承角色「%s」，无需重复继承", targetRole)
	case InheritKindIndirect:
		if topParent != "" {
			return fmt.Sprintf("角色「%s」已通过「%s」间接继承，无需重复继承；如需调整请先解除与「%s」的继承", targetRole, topParent, topParent)
		}

		return fmt.Sprintf("角色「%s」已通过继承链间接获得，无需重复继承", targetRole)
	default:
		return fmt.Sprintf("已继承角色「%s」", targetRole)
	}
}

// FormatIndirectUnlinkMsg 尝试解除间接继承时的提示。
func FormatIndirectUnlinkMsg(targetRole, topParent string) string {
	if topParent != "" {
		return fmt.Sprintf("「%s」为间接继承，无法直接解除；请先解除与顶层父角色「%s」的继承关系", targetRole, topParent)
	}

	return fmt.Sprintf("「%s」为间接继承，无法直接解除，请先解除对应的直接父角色继承", targetRole)
}

// RoleInheritTreeNode 角色继承树节点。
type RoleInheritTreeNode struct {
	ID       uint                   `json:"id"`
	Name     string                 `json:"name"`
	Desc     string                 `json:"desc"`
	Children []*RoleInheritTreeNode `json:"children,omitempty"`
}

// roleNodeInfo 建树期间使用的角色摘要。
type roleNodeInfo struct {
	ID   uint
	Name string
	Desc string
}

// BuildRoleInheritTree 依据角色清单与 Casbin g 分组策略构建角色继承树。
//
// 语义与原 api/rbac_api/role_inherit_tree.go 的 buildRoleInheritTree 完全一致：
//   - 跳过残缺策略行、user: 前缀主体与不在角色表中的名字；
//   - 同一父节点的子节点按名称排序，根节点集合按名称排序；
//   - 建树期间用路径集合防止继承环导致的无限递归；
//   - 存在继承环或无根组件时补齐不可达角色，保证全量输出。
func BuildRoleInheritTree(roles []*Role, groupingPolicies [][]string) []*RoleInheritTreeNode {
	if len(roles) == 0 {
		return []*RoleInheritTreeNode{}
	}

	nodeInfo := make(map[string]roleNodeInfo, len(roles))

	for _, role := range roles {
		if role == nil {
			continue
		}

		name := strings.TrimSpace(role.Name)
		if name == "" {
			continue
		}

		nodeInfo[name] = roleNodeInfo{ID: role.ID, Name: name, Desc: role.Desc}
	}

	childrenMap := make(map[string][]string)
	childSet := make(map[string]struct{})

	for _, p := range groupingPolicies {
		if len(p) < 2 {
			continue
		}

		child := strings.TrimSpace(p[0])

		parent := strings.TrimSpace(p[1])
		if child == "" || parent == "" {
			continue
		}

		if strings.HasPrefix(child, userSubjectPrefix) || strings.HasPrefix(parent, userSubjectPrefix) {
			continue
		}

		if _, ok := nodeInfo[child]; !ok {
			continue
		}

		if _, ok := nodeInfo[parent]; !ok {
			continue
		}

		childrenMap[parent] = append(childrenMap[parent], child)
		childSet[child] = struct{}{}
	}

	for parent := range childrenMap {
		slices.Sort(childrenMap[parent])
	}

	roots := make([]string, 0)

	for name := range nodeInfo {
		if _, ok := childSet[name]; ok {
			continue
		}

		roots = append(roots, name)
	}

	sort.Strings(roots)

	if len(roots) == 0 {
		for name := range nodeInfo {
			roots = append(roots, name)
		}

		sort.Strings(roots)
	}

	visited := make(map[string]struct{})

	var build func(name string, path map[string]struct{}) *RoleInheritTreeNode

	build = func(name string, path map[string]struct{}) *RoleInheritTreeNode {
		info, ok := nodeInfo[name]
		if !ok {
			return nil
		}
		// 防止继承环导致递归死循环
		if _, ok := path[name]; ok {
			return nil
		}

		visited[name] = struct{}{}
		node := &RoleInheritTreeNode{
			ID:   info.ID,
			Name: info.Name,
			Desc: info.Desc,
		}

		path[name] = struct{}{}
		if children, ok := childrenMap[name]; ok {
			for _, child := range children {
				if childNode := build(child, path); childNode != nil {
					node.Children = append(node.Children, childNode)
				}
			}
		}

		delete(path, name)

		return node
	}

	result := make([]*RoleInheritTreeNode, 0, len(roots))
	for _, root := range roots {
		if node := build(root, map[string]struct{}{}); node != nil {
			result = append(result, node)
		}
	}

	if len(visited) < len(nodeInfo) {
		// 存在继承环或无根组件时，补齐不可达角色，确保全量输出
		missing := make([]string, 0)

		for name := range nodeInfo {
			if _, ok := visited[name]; ok {
				continue
			}

			missing = append(missing, name)
		}

		sort.Strings(missing)

		for _, name := range missing {
			if node := build(name, map[string]struct{}{}); node != nil {
				result = append(result, node)
			}
		}
	}

	return result
}

// FilterRoleInheritTree 只保留包含 target 的子树（target 自身及其后代全量保留）。
func FilterRoleInheritTree(nodes []*RoleInheritTreeNode, target string) []*RoleInheritTreeNode {
	if len(nodes) == 0 {
		return []*RoleInheritTreeNode{}
	}

	result := make([]*RoleInheritTreeNode, 0)

	for _, node := range nodes {
		if node == nil {
			continue
		}

		if pruned, ok := pruneRoleInheritTree(node, target); ok {
			result = append(result, pruned)
		}
	}

	return result
}

// pruneRoleInheritTree 自上而下裁剪，返回是否命中 target。
func pruneRoleInheritTree(node *RoleInheritTreeNode, target string) (*RoleInheritTreeNode, bool) {
	if node == nil {
		return nil, false
	}

	if node.Name == target {
		return node, true
	}

	if len(node.Children) == 0 {
		return nil, false
	}

	kept := make([]*RoleInheritTreeNode, 0)

	for _, child := range node.Children {
		if pruned, ok := pruneRoleInheritTree(child, target); ok {
			kept = append(kept, pruned)
		}
	}

	if len(kept) == 0 {
		return nil, false
	}

	copied := *node
	copied.Children = kept

	return &copied, true
}
