package rbac

import (
	"context"
	"sort"
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	"go.uber.org/zap"
)

// PermissionService 权限用例服务（原 api/rbac_api 中 permission/resource 相关处理器的业务部分）。
type PermissionService struct {
	roles     domain.RoleRepository
	roleMenus domain.RoleMenuRepository
	resources domain.ApiResourceRepository
	enforcer  Enforcer
	graph     *RoleGraph
	evaluator *domain.PermissionEvaluator
}

// NewPermissionService 构造权限用例服务。
func NewPermissionService(
	roles domain.RoleRepository,
	roleMenus domain.RoleMenuRepository,
	resources domain.ApiResourceRepository,
	enforcer Enforcer,
	graph *RoleGraph,
	evaluator *domain.PermissionEvaluator,
) *PermissionService {
	return &PermissionService{
		roles:     roles,
		roleMenus: roleMenus,
		resources: resources,
		enforcer:  enforcer,
		graph:     graph,
		evaluator: evaluator,
	}
}

// Assign 为角色追加权限：拒绝列表内部冗余与已被有效权限覆盖的项。
func (s *PermissionService) Assign(ctx context.Context, params AssignPermissionParams) error {
	perms, err := domain.CollectPermissions(params.Permissions)
	if err != nil {
		return err
	}

	// 验证角色是否存在
	role, err := s.roles.FindByName(ctx, params.RoleName)
	if err != nil || role == nil {
		return domain.ErrRoleNotFound
	}

	// 列表内部冗余（如同时含 path * 与 path GET）
	if err := s.evaluator.CheckInternalRedundancy(perms); err != nil {
		return err
	}

	// 已有有效权限（直接 + 继承）覆盖则拒绝
	effective, err := s.graph.EffectivePermissionItems(params.RoleName)
	if err != nil {
		return domain.ErrQueryPermissionFailed
	}

	if err := s.evaluator.CheckAlreadyOwned(effective, perms); err != nil {
		return err
	}

	for _, p := range perms {
		if _, err := s.enforcer.AddPolicy(params.RoleName, p.Resource, p.Action); err != nil {
			return domain.ErrAddFailed
		}
	}

	if err := s.enforcer.SavePolicy(); err != nil {
		return domain.ErrSaveFailed
	}

	return nil
}

// Remove 移除角色的指定直接权限。
func (s *PermissionService) Remove(ctx context.Context, params AssignPermissionParams) error {
	perms, err := domain.CollectPermissions(params.Permissions)
	if err != nil {
		return err
	}

	// 验证角色是否存在
	role, err := s.roles.FindByName(ctx, params.RoleName)
	if err != nil || role == nil {
		return domain.ErrRoleNotFound
	}

	for _, p := range perms {
		if _, err := s.enforcer.RemovePolicy(params.RoleName, p.Resource, p.Action); err != nil {
			zap.L().Error("移除失败", zap.Error(err))

			return domain.ErrRemoveFailed
		}
	}

	if err := s.enforcer.SavePolicy(); err != nil {
		zap.L().Error("保存失败", zap.Error(err))

		return domain.ErrSaveFailed
	}

	return nil
}

// RolePermissions 获取角色权限（直接权限 + 有效权限 + 覆盖的具体 API）。
func (s *PermissionService) RolePermissions(ctx context.Context, roleName string) (RolePermissionsResp, error) {
	role, err := s.roles.FindByName(ctx, roleName)
	if err != nil || role == nil {
		return RolePermissionsResp{}, domain.ErrRoleNotFound
	}

	directRows, err := s.enforcer.GetPermissionsForUser(roleName)
	if err != nil {
		zap.L().Error("获取角色直接权限失败", zap.Error(err))

		return RolePermissionsResp{}, domain.ErrGetPermissionFailed
	}

	effectiveRows, err := s.enforcer.GetImplicitPermissionsForUser(roleName)
	if err != nil {
		zap.L().Error("获取角色有效权限失败", zap.Error(err))

		return RolePermissionsResp{}, domain.ErrGetPermissionFailed
	}

	directItems := domain.NormalizeCasbinPolicyRows(directRows)

	directParents, err := s.graph.DirectParents(roleName)
	if err != nil {
		zap.L().Error("获取直接父角色失败", zap.Error(err))

		return RolePermissionsResp{}, domain.ErrGetPermissionFailed
	}

	directParentSet := make(map[string]struct{}, len(directParents))
	for _, p := range directParents {
		directParentSet[strings.TrimSpace(p)] = struct{}{}
	}

	effective := make([]EffectivePermissionItem, 0, len(effectiveRows))
	// 覆盖计算只需 path+method 去重；列表展示必须按 owner+path+method 保留，
	// 否则「123」与「普通用户」同持 /v1/common/** 时只会显示其中一个。
	seenOwnerPolicy := make(map[string]struct{})
	seenPathAction := make(map[string]struct{})
	effectiveItems := make([]domain.PermissionItem, 0, len(effectiveRows))

	for _, row := range effectiveRows {
		if len(row) < 3 {
			continue
		}

		owner := strings.TrimSpace(row[0])
		resource := strings.TrimSpace(row[1])

		action := strings.TrimSpace(row[2])
		if resource == "" || action == "" {
			continue
		}

		ownerKey := owner + "\x00" + resource + "\x00" + strings.ToUpper(action)
		if _, ok := seenOwnerPolicy[ownerKey]; ok {
			continue
		}

		seenOwnerPolicy[ownerKey] = struct{}{}

		item := EffectivePermissionItem{
			Resource: resource,
			Action:   action,
			Owner:    owner,
		}

		// 按策略持有者判定来源，不能因为当前角色也有同 path 就把继承策略标成 direct
		if owner == "" || owner == roleName {
			item.Source = RolePermissionSourceDirect
		} else if _, ok := directParentSet[owner]; ok {
			item.Source = RolePermissionSourceInheritDirect
			item.TopParent = owner
		} else {
			top, kind, ferr := s.graph.FindTopParentForOwner(roleName, owner)
			if ferr != nil {
				zap.L().Warn("解析继承顶层父角色失败", zap.Error(ferr), zap.String("owner", owner))
			}

			if kind == domain.InheritKindDirect {
				item.Source = RolePermissionSourceInheritDirect
				item.TopParent = top
			} else {
				item.Source = RolePermissionSourceInheritIndirect
				item.TopParent = top
			}
		}

		effective = append(effective, item)

		pathKey := resource + "\x00" + strings.ToUpper(action)
		if _, ok := seenPathAction[pathKey]; !ok {
			seenPathAction[pathKey] = struct{}{}
			effectiveItems = append(effectiveItems, domain.PermissionItem{Resource: resource, Action: action})
		}
	}

	// 补上直接策略里可能未被 implicit 返回的项（极少见）
	for _, p := range directItems {
		pathKey := p.Resource + "\x00" + strings.ToUpper(p.Action)

		ownerKey := roleName + "\x00" + pathKey
		if _, ok := seenOwnerPolicy[ownerKey]; ok {
			continue
		}

		seenOwnerPolicy[ownerKey] = struct{}{}
		effective = append(effective, EffectivePermissionItem{
			Resource: p.Resource,
			Action:   p.Action,
			Owner:    roleName,
			Source:   RolePermissionSourceDirect,
		})

		if _, ok := seenPathAction[pathKey]; !ok {
			seenPathAction[pathKey] = struct{}{}
			effectiveItems = append(effectiveItems, p)
		}
	}

	// 计算资源表中被有效权限覆盖的具体 API
	resources, err := s.resources.ListPathMethod(ctx)
	if err != nil {
		zap.L().Error("读取资源表失败", zap.Error(err))

		return RolePermissionsResp{}, domain.ErrGetPermissionFailed
	}

	covered := make([]string, 0)

	for _, r := range resources {
		path := strings.TrimSpace(r.Path)

		method := strings.TrimSpace(r.Method)
		if path == "" || method == "" {
			continue
		}

		if s.evaluator.IsCoveredBy(effectiveItems, path, method) {
			covered = append(covered, strings.ToUpper(method)+":"+path)
		}
	}

	return RolePermissionsResp{
		Direct:    directRows,
		Effective: effective,
		Covered:   covered,
	}, nil
}

// UpdateRolePermissions 用给定列表整体替换角色的直接权限。
//
// BreakInheritance 为 true 时先清除该角色的全部继承关系与继承菜单，
// 用于「取消勾选通配符/继承覆盖的具体 API」场景。
func (s *PermissionService) UpdateRolePermissions(ctx context.Context, params UpdateRolePermissionsParams) error {
	role, err := s.roles.FindByID(ctx, params.RoleID)
	if err != nil || role == nil {
		return domain.ErrRoleNotFound
	}

	perms, err := domain.NormalizeReplacementPermissions(params.Permissions)
	if err != nil {
		return err
	}

	// 列表内部冗余
	if err := s.evaluator.CheckInternalRedundancy(perms); err != nil {
		return err
	}

	if params.BreakInheritance {
		if err := s.clearRoleInheritance(ctx, role); err != nil {
			return err
		}
	} else {
		// 未解除继承时：新写入的直接权限不得已被继承权限覆盖
		direct, err := s.graph.DirectPermissionItems(role.Name)
		if err != nil {
			return domain.ErrQueryPermissionFailed
		}

		effective, err := s.graph.EffectivePermissionItems(role.Name)
		if err != nil {
			return domain.ErrQueryPermissionFailed
		}

		inherited := domain.FilterInheritedPermissions(direct, effective)
		if err := s.evaluator.CheckAlreadyOwned(inherited, perms); err != nil {
			return err
		}
	}

	// 清空原有直接权限
	if _, err := s.enforcer.RemoveFilteredPolicy(0, role.Name); err != nil {
		zap.L().Error("清理角色权限失败", zap.Error(err))

		return domain.ErrUpdateFailed
	}

	for _, p := range perms {
		if _, err := s.enforcer.AddPolicy(role.Name, p.Resource, p.Action); err != nil {
			zap.L().Error("添加角色权限失败", zap.Error(err))

			return domain.ErrUpdateFailed
		}
	}

	if err := s.enforcer.SavePolicy(); err != nil {
		zap.L().Error("保存角色权限失败", zap.Error(err))

		return domain.ErrUpdateFailed
	}

	return nil
}

// clearRoleInheritance 清除角色的全部 Casbin 继承关系与继承菜单。
func (s *PermissionService) clearRoleInheritance(ctx context.Context, role *domain.Role) error {
	parents, err := s.enforcer.GetRolesForUser(role.Name)
	if err != nil {
		zap.L().Error("读取角色继承失败", zap.Error(err))

		return domain.ErrBreakInheritanceFailed
	}

	for _, parent := range parents {
		if _, err := s.enforcer.DeleteRoleForUser(role.Name, parent); err != nil {
			zap.L().Error("移除角色继承失败", zap.Error(err), zap.String("parent", parent))

			return domain.ErrBreakInheritanceFailed
		}
	}

	if err := s.roleMenus.DeleteInheritedByRole(ctx, role.ID); err != nil {
		zap.L().Error("移除继承菜单失败", zap.Error(err))

		return domain.ErrBreakInheritanceFailed
	}

	return nil
}

// WildcardList 返回后端所有已配置的通配符权限列表（跨角色去重后按资源、方法排序）。
func (s *PermissionService) WildcardList() ([]WildcardPermissionItem, error) {
	policies, err := s.enforcer.GetPolicy()
	if err != nil {
		zap.L().Error("读取权限策略失败", zap.Error(err))

		return nil, domain.ErrGetWildcardFailed
	}

	seen := make(map[string]struct{})
	items := make([]WildcardPermissionItem, 0)

	for _, p := range policies {
		if len(p) < 3 {
			continue
		}

		resource := strings.TrimSpace(p[1])

		action := strings.TrimSpace(p[2])
		if resource == "" || action == "" {
			continue
		}

		if !domain.IsWildcardPermission(resource, action) {
			continue
		}

		key := resource + "\x00" + strings.ToUpper(action)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		items = append(items, WildcardPermissionItem{
			Resource: resource,
			Action:   action,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Resource == items[j].Resource {
			return items[i].Action < items[j].Action
		}

		return items[i].Resource < items[j].Resource
	})

	return items, nil
}

// ResourceList 返回可用于权限配置的资源列表。
func (s *PermissionService) ResourceList(ctx context.Context) ([]APIResourceItem, error) {
	resources, err := s.resources.ListOrdered(ctx)
	if err != nil {
		zap.L().Error("读取资源表失败", zap.Error(err))

		return nil, domain.ErrGetResourceFailed
	}

	items := make([]APIResourceItem, 0, len(resources))

	for _, item := range resources {
		items = append(items, APIResourceItem{
			Path:    item.Path,
			Method:  item.Method,
			Summary: item.Summary,
			Tags:    domain.DecodeResourceTags(item.Tags),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Method < items[j].Method
		}

		return items[i].Path < items[j].Path
	})

	return items, nil
}
