package rbac

import "strings"

// PermissionItem 权限项（资源路径 + 操作方法）。
//
// 领域内部使用的纯值对象；HTTP 请求体里的同名结构体带 binding/label tag，保留在 handler 包。
type PermissionItem struct {
	Resource string
	Action   string
}

// PathMatcher 路径通配匹配端口。
//
// 语义必须与 conf/rbac_model.conf 中 matcher 使用的 globMatch 完全一致
// （globMatch(request, policyPattern)），实现落在 infrastructure，避免领域层依赖 Casbin。
type PathMatcher interface {
	// Match 判断请求路径 request 是否匹配策略模式 pattern。
	Match(request, pattern string) (bool, error)
}

// PermissionEvaluator 权限覆盖判定器（原 api/rbac_api/permission_helpers.go）。
type PermissionEvaluator struct {
	matcher PathMatcher
}

// NewPermissionEvaluator 构造权限覆盖判定器。
func NewPermissionEvaluator(matcher PathMatcher) *PermissionEvaluator {
	return &PermissionEvaluator{matcher: matcher}
}

// PathCovers 判断 existingPattern 是否覆盖 target（target 可为具体路径或通配路径）。
func (e *PermissionEvaluator) PathCovers(existingPattern, target string) bool {
	existingPattern = strings.TrimSpace(existingPattern)
	target = strings.TrimSpace(target)

	if existingPattern == "" || target == "" {
		return false
	}

	if existingPattern == target {
		return true
	}

	if e == nil || e.matcher == nil {
		return false
	}
	// 与 Casbin 模型一致：globMatch(request, policyPattern)
	ok, err := e.matcher.Match(target, existingPattern)

	return err == nil && ok
}

// PermissionCovers 判断已有权限是否覆盖待添加权限。
func (e *PermissionEvaluator) PermissionCovers(existRes, existAct, newRes, newAct string) bool {
	return e.PathCovers(existRes, newRes) && ActionCovers(existAct, newAct)
}

// ItemCovers 基于 PermissionItem 判断覆盖关系。
func (e *PermissionEvaluator) ItemCovers(existing, target PermissionItem) bool {
	return e.PermissionCovers(existing.Resource, existing.Action, target.Resource, target.Action)
}

// CheckAlreadyOwned 检查 targets 是否已被 owned 中的权限覆盖。
// 若覆盖则返回 ErrPermissionAlreadyOwned。
func (e *PermissionEvaluator) CheckAlreadyOwned(owned, targets []PermissionItem) error {
	for _, t := range targets {
		for _, o := range owned {
			if e.ItemCovers(o, t) {
				return ErrPermissionAlreadyOwned
			}
		}
	}

	return nil
}

// CheckInternalRedundancy 检查列表内部是否存在“一项覆盖另一项”的冗余。
func (e *PermissionEvaluator) CheckInternalRedundancy(items []PermissionItem) error {
	for i := range items {
		for j := range items {
			if i == j {
				continue
			}

			if e.ItemCovers(items[j], items[i]) {
				return ErrPermissionAlreadyOwned
			}
		}
	}

	return nil
}

// IsCoveredBy 判断 path+method 是否被权限列表覆盖。
func (e *PermissionEvaluator) IsCoveredBy(perms []PermissionItem, path, method string) bool {
	for _, p := range perms {
		if e.PermissionCovers(p.Resource, p.Action, path, method) {
			return true
		}
	}

	return false
}

// ActionCovers 判断 existingAction 是否覆盖 targetAction（* 覆盖任意方法）。
func ActionCovers(existingAction, targetAction string) bool {
	existingAction = strings.TrimSpace(existingAction)
	targetAction = strings.TrimSpace(targetAction)

	if existingAction == "" || targetAction == "" {
		return false
	}

	return existingAction == "*" || strings.EqualFold(existingAction, targetAction)
}

// IsWildcardPermission 判断是否为通配符权限（路径含 * 或方法为 *）。
func IsWildcardPermission(resource, action string) bool {
	return strings.Contains(resource, "*") || action == "*"
}

// NormalizeCasbinPolicyRows 将 Casbin 策略行转为 PermissionItem（跳过残缺行与空字段）。
//
// 原实现在 create_role.go 与 permission_helpers.go 中各有一份完全相同的副本
// （normalizeCasbinPermissions / normalizeCasbinPolicyRows），此处合并为一处。
func NormalizeCasbinPolicyRows(rows [][]string) []PermissionItem {
	items := make([]PermissionItem, 0, len(rows))

	for _, row := range rows {
		if len(row) < 3 {
			continue
		}

		resource := strings.TrimSpace(row[1])

		action := strings.TrimSpace(row[2])
		if resource == "" || action == "" {
			continue
		}

		items = append(items, PermissionItem{Resource: resource, Action: action})
	}

	return items
}

// FilterInheritedPermissions 从 effective 中去掉 direct，得到纯继承权限。
func FilterInheritedPermissions(direct, effective []PermissionItem) []PermissionItem {
	directSet := make(map[string]struct{}, len(direct))
	for _, p := range direct {
		directSet[p.Resource+"\x00"+strings.ToUpper(p.Action)] = struct{}{}
	}

	inherited := make([]PermissionItem, 0)

	seen := make(map[string]struct{})

	for _, p := range effective {
		key := p.Resource + "\x00" + strings.ToUpper(p.Action)
		if _, ok := directSet[key]; ok {
			continue
		}

		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}

		inherited = append(inherited, p)
	}

	return inherited
}

// DedupePermissions 按「资源 + 空格 + 方法」去重，保持首次出现顺序。
func DedupePermissions(items []PermissionItem) []PermissionItem {
	seen := make(map[string]struct{})

	result := make([]PermissionItem, 0, len(items))
	for _, p := range items {
		key := p.Resource + " " + p.Action
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}

		result = append(result, p)
	}

	return result
}

// NormalizeNewPermissions 规范化「创建角色」时携带的权限列表。
//
// 与原 create_role.go 的 normalizePermissions 一致：空列表返回 nil，不做去重。
func NormalizeNewPermissions(items []PermissionItem) ([]PermissionItem, error) {
	if len(items) == 0 {
		return nil, nil
	}

	return trimPermissions(items)
}

// CollectPermissions 规范化「分配/移除权限」请求携带的权限列表。
//
// 与原 add_permission.go 的 collectPermissions 一致：空列表视为非法。
func CollectPermissions(items []PermissionItem) ([]PermissionItem, error) {
	if len(items) == 0 {
		return nil, ErrPermissionsRequired
	}

	return trimPermissions(items)
}

// NormalizeReplacementPermissions 规范化「替换角色权限」请求携带的权限列表。
//
// 与原 update_role_permissions.go 的 normalizePermissionList 一致：空列表返回空切片，并对结果去重。
func NormalizeReplacementPermissions(items []PermissionItem) ([]PermissionItem, error) {
	if len(items) == 0 {
		return []PermissionItem{}, nil
	}

	result, err := trimPermissions(items)
	if err != nil {
		return nil, err
	}

	return DedupePermissions(result), nil
}

// trimPermissions 去除资源与方法的首尾空白；任一为空即视为参数不完整。
func trimPermissions(items []PermissionItem) ([]PermissionItem, error) {
	result := make([]PermissionItem, 0, len(items))

	for _, p := range items {
		resource := strings.TrimSpace(p.Resource)

		action := strings.TrimSpace(p.Action)
		if resource == "" || action == "" {
			return nil, ErrPermissionParamIncomplete
		}

		result = append(result, PermissionItem{Resource: resource, Action: action})
	}

	return result, nil
}
