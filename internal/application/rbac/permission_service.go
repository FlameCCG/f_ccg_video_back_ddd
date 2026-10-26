package rbac

import (
	"context"
	"errors"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/logctx"
	"sort"
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	"go.uber.org/zap"
)

// PermissionService 权限用例服务（原 api/rbac_api 中 permission/resource 相关处理器的业务部分）。
type PermissionService struct {
	roles     domain.RoleRepository
	roleMenus domain.RoleMenuRepository
	resources domain.APIResourceRepository
	enforcer  Enforcer
	graph     *RoleGraph
	evaluator *domain.PermissionEvaluator
	logger    *zap.Logger
}

// NewPermissionService 构造权限用例服务。
func NewPermissionService(
	roles domain.RoleRepository,
	roleMenus domain.RoleMenuRepository,
	resources domain.APIResourceRepository,
	enforcer Enforcer,
	graph *RoleGraph,
	evaluator *domain.PermissionEvaluator,
	logger *zap.Logger,
) *PermissionService {
	return &PermissionService{
		roles:     roles,
		roleMenus: roleMenus,
		resources: resources,
		enforcer:  enforcer,
		graph:     graph,
		evaluator: evaluator,
		logger:    logger,
	}
}

// Assign 为角色追加权限：拒绝列表内部冗余与已被有效权限覆盖的项。
func (s *PermissionService) Assign(ctx context.Context, params AssignPermissionParams) (result error) {
	scoped, finish, err := beginPolicyChange(s.enforcer)
	if err != nil {
		return err
	}
	defer finish(&result)

	service := *s
	service.enforcer = scoped
	s = &service
	s.graph = NewRoleGraph(scoped)

	perms, err := domain.CollectPermissions(params.Permissions)
	if err != nil {
		return err
	}

	// 验证角色是否存在
	role, err := s.roles.FindByName(ctx, params.RoleName)
	if err != nil || role == nil {
		return apperr.Wrap("application/rbac/permission_service", domain.ErrRoleNotFound, err)
	}

	// 列表内部冗余（如同时含 path * 与 path GET）
	if err := s.evaluator.CheckInternalRedundancy(perms); err != nil {
		return err
	}

	// 已有有效权限（直接 + 继承）覆盖则拒绝
	effective, err := s.graph.EffectivePermissionItems(params.RoleName)
	if err != nil {
		return apperr.Wrap("application/rbac/permission_service", domain.ErrQueryPermissionFailed, err)
	}

	if err := s.evaluator.CheckAlreadyOwned(effective, perms); err != nil {
		return err
	}

	for _, p := range perms {
		if _, err := s.enforcer.AddPolicy(params.RoleName, p.Resource, p.Action); err != nil {
			return apperr.Wrap("application/rbac/permission_service", domain.ErrAddFailed, err)
		}
	}

	if err := s.enforcer.SavePolicy(); err != nil {
		return apperr.Wrap("application/rbac/permission_service", domain.ErrSaveFailed, err)
	}

	return nil
}

// Remove 移除角色的指定直接权限。
func (s *PermissionService) Remove(ctx context.Context, params AssignPermissionParams) (result error) {
	scoped, finish, err := beginPolicyChange(s.enforcer)
	if err != nil {
		return err
	}
	defer finish(&result)

	service := *s
	service.enforcer = scoped
	s = &service
	s.graph = NewRoleGraph(scoped)

	perms, err := domain.CollectPermissions(params.Permissions)
	if err != nil {
		return err
	}

	// 验证角色是否存在
	role, err := s.roles.FindByName(ctx, params.RoleName)
	if err != nil || role == nil {
		return apperr.Wrap("application/rbac/permission_service", domain.ErrRoleNotFound, err)
	}

	for _, p := range perms {
		if _, err := s.enforcer.RemovePolicy(params.RoleName, p.Resource, p.Action); err != nil {
			return apperr.Wrap("application/rbac/permission_service", domain.ErrRemoveFailed, err)
		}
	}

	if err := s.enforcer.SavePolicy(); err != nil {
		return apperr.Wrap("application/rbac/permission_service", domain.ErrSaveFailed, err)
	}

	return nil
}

// RolePermissions 获取角色权限（直接权限 + 有效权限 + 覆盖的具体 API）。
func (s *PermissionService) RolePermissions(ctx context.Context, roleName string) (RolePermissionsResp, error) {
	role, err := s.roles.FindByName(ctx, roleName)
	if err != nil || role == nil {
		return RolePermissionsResp{}, apperr.Wrap("application/rbac/permission_service", domain.ErrRoleNotFound, err)
	}

	directRows, err := s.enforcer.GetPermissionsForUser(roleName)
	if err != nil {
		return RolePermissionsResp{}, apperr.Wrap("application/rbac/permission_service", domain.ErrGetPermissionFailed, err)
	}

	effectiveRows, err := s.enforcer.GetImplicitPermissionsForUser(roleName)
	if err != nil {
		return RolePermissionsResp{}, apperr.Wrap("application/rbac/permission_service", domain.ErrGetPermissionFailed, err)
	}

	directItems := domain.NormalizeCasbinPolicyRows(directRows)

	directParents, err := s.graph.DirectParents(roleName)
	if err != nil {
		return RolePermissionsResp{}, apperr.Wrap("application/rbac/permission_service", domain.ErrGetPermissionFailed, err)
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

		item = s.resolvePermissionSource(ctx, roleName, directParentSet, item)

		effective = append(effective, item)

		pathKey := resource + "\x00" + strings.ToUpper(action)
		if _, ok := seenPathAction[pathKey]; !ok {
			seenPathAction[pathKey] = struct{}{}

			effectiveItems = append(effectiveItems, domain.PermissionItem{Resource: resource, Action: action})
		}
	}

	// 补上直接策略里可能未被 implicit 返回的项（极少见）
	return finishPermissionServiceRolePermissions(ctx, s, roleName, directRows, directItems, &effective, seenOwnerPolicy, seenPathAction, &effectiveItems)
}

// UpdateRolePermissions 用给定列表整体替换角色的直接权限。
//
// BreakInheritance 为 true 时先清除该角色的全部继承关系与继承菜单，
// 用于「取消勾选通配符/继承覆盖的具体 API」场景。
func (s *PermissionService) UpdateRolePermissions(ctx context.Context, params UpdateRolePermissionsParams) (result error) {
	scoped, finish, err := beginPolicyChange(s.enforcer)
	if err != nil {
		return err
	}
	defer finish(&result)

	service := *s
	service.enforcer = scoped
	s = &service
	s.graph = NewRoleGraph(scoped)

	role, err := s.roles.FindByID(ctx, params.RoleID)
	if err != nil || role == nil {
		return apperr.Wrap("application/rbac/permission_service", domain.ErrRoleNotFound, err)
	}

	perms, err := domain.NormalizeReplacementPermissions(params.Permissions)
	if err != nil {
		return err
	}

	// 列表内部冗余
	if err := s.evaluator.CheckInternalRedundancy(perms); err != nil {
		return err
	}

	return finishPermissionServiceUpdateRolePermissions(ctx, s, params, role, perms)
}

// clearRoleInheritance 清除角色的全部 Casbin 继承关系与继承菜单。
func (s *PermissionService) clearRoleInheritance(ctx context.Context, role *domain.Role) error {
	parents, err := s.enforcer.GetRolesForUser(role.Name)
	if err != nil {
		return apperr.Wrap("application/rbac/permission_service", domain.ErrBreakInheritanceFailed, err)
	}

	for _, parent := range parents {
		if _, err := s.enforcer.DeleteRoleForUser(role.Name, parent); err != nil {
			return apperr.Wrap("application/rbac/permission_service", domain.ErrBreakInheritanceFailed, err)
		}
	}

	if err := s.roleMenus.DeleteInheritedByRole(ctx, role.ID); err != nil {
		return apperr.Wrap("application/rbac/permission_service", domain.ErrBreakInheritanceFailed, err)
	}

	return nil
}

// WildcardList 返回后端所有已配置的通配符权限列表（跨角色去重后按资源、方法排序）。
func (s *PermissionService) WildcardList() ([]WildcardPermissionItem, error) {
	policies, err := s.enforcer.GetPolicy()
	if err != nil {
		return nil, apperr.Wrap("application/rbac/permission_service", domain.ErrGetWildcardFailed, err)
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
		return nil, apperr.Wrap("application/rbac/permission_service", domain.ErrGetResourceFailed, err)
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

func finishPermissionServiceRolePermissions(ctx context.Context, s *PermissionService, roleName string, directRows [][]string, directItems []domain.PermissionItem, effective *[]EffectivePermissionItem, seenOwnerPolicy, seenPathAction map[string]struct{}, effectiveItems *[]domain.PermissionItem) (RolePermissionsResp, error) {
	var err error

	for _, p := range directItems {
		pathKey := p.Resource + "\x00" + strings.ToUpper(p.Action)

		ownerKey := roleName + "\x00" + pathKey
		if _, ok := seenOwnerPolicy[ownerKey]; ok {
			continue
		}

		seenOwnerPolicy[ownerKey] = struct{}{}

		(*effective) = append((*effective), EffectivePermissionItem{
			Resource: p.Resource,
			Action:   p.Action,
			Owner:    roleName,
			Source:   RolePermissionSourceDirect,
		})

		if _, ok := seenPathAction[pathKey]; !ok {
			seenPathAction[pathKey] = struct{}{}

			(*effectiveItems) = append((*effectiveItems), p)
		}
	}

	// 计算资源表中被有效权限覆盖的具体 API
	resources, err := s.resources.ListPathMethod(ctx)
	if err != nil {
		return RolePermissionsResp{}, apperr.Wrap("application/rbac/permission_service", domain.ErrGetPermissionFailed, err)
	}

	covered := make([]string, 0)

	for _, r := range resources {
		path := strings.TrimSpace(r.Path)

		method := strings.TrimSpace(r.Method)
		if path == "" || method == "" {
			continue
		}

		if s.evaluator.IsCoveredBy((*effectiveItems), path, method) {
			covered = append(covered, strings.ToUpper(method)+":"+path)
		}
	}

	return RolePermissionsResp{
		Direct:    directRows,
		Effective: (*effective),
		Covered:   covered,
	}, nil
}

func finishPermissionServiceUpdateRolePermissions(ctx context.Context, s *PermissionService, params UpdateRolePermissionsParams, role *domain.Role, perms []domain.PermissionItem) (result error) {
	if params.BreakInheritance {
		previousMenus, err := s.roleMenus.InheritedMenuIDs(ctx, role.ID)
		if err != nil {
			return apperr.Wrap("application/rbac/permission_service", domain.ErrBreakInheritanceFailed, err)
		}
		defer s.restoreInheritedMenusOnFailure(ctx, role.ID, previousMenus, &result)

		if err := s.clearRoleInheritance(ctx, role); err != nil {
			return err
		}
	} else if err := s.ensurePermissionsNotInherited(role.Name, perms); err != nil {
		return err
	}

	// 清空原有直接权限
	if _, err := s.enforcer.RemoveFilteredPolicy(0, role.Name); err != nil {
		return apperr.Wrap("application/rbac/permission_service", domain.ErrUpdateFailed, err)
	}

	for _, p := range perms {
		if _, err := s.enforcer.AddPolicy(role.Name, p.Resource, p.Action); err != nil {
			return apperr.Wrap("application/rbac/permission_service", domain.ErrUpdateFailed, err)
		}
	}

	if err := s.enforcer.SavePolicy(); err != nil {
		return apperr.Wrap("application/rbac/permission_service", domain.ErrUpdateFailed, err)
	}

	return nil
}

func (s *PermissionService) resolvePermissionSource(ctx context.Context, roleName string, directParents map[string]struct{}, item EffectivePermissionItem) EffectivePermissionItem {
	if item.Owner == "" || item.Owner == roleName {
		item.Source = RolePermissionSourceDirect
		return item
	}

	if _, ok := directParents[item.Owner]; ok {
		item.Source = RolePermissionSourceInheritDirect
		item.TopParent = item.Owner

		return item
	}

	top, kind, err := s.graph.FindTopParentForOwner(roleName, item.Owner)
	if err != nil {
		logctx.With(ctx, s.logger).Warn("解析继承顶层父角色失败", zap.String("event", "rbac.permission_service.role_permissions.degraded"), zap.String("operation", "rbac.permission_service.role_permissions"), zap.Error(err), zap.String("owner", item.Owner))
	}

	item.Source = RolePermissionSourceInheritIndirect

	item.TopParent = top
	if kind == domain.InheritKindDirect {
		item.Source = RolePermissionSourceInheritDirect
	}

	return item
}

func (s *PermissionService) restoreInheritedMenusOnFailure(ctx context.Context, roleID uint, previous map[uint]struct{}, result *error) {
	if *result == nil {
		return
	}

	ids := make([]uint, 0, len(previous))
	for id := range previous {
		ids = append(ids, id)
	}

	if err := s.roleMenus.CreateIgnoreConflict(context.WithoutCancel(ctx), domain.NewInheritedRoleMenus(roleID, ids)); err != nil {
		*result = errors.Join(*result, err)
	}
}

func (s *PermissionService) ensurePermissionsNotInherited(roleName string, perms []domain.PermissionItem) error {
	direct, err := s.graph.DirectPermissionItems(roleName)
	if err != nil {
		return apperr.Wrap("application/rbac/permission_service", domain.ErrQueryPermissionFailed, err)
	}

	effective, err := s.graph.EffectivePermissionItems(roleName)
	if err != nil {
		return apperr.Wrap("application/rbac/permission_service", domain.ErrQueryPermissionFailed, err)
	}

	return s.evaluator.CheckAlreadyOwned(domain.FilterInheritedPermissions(direct, effective), perms)
}
