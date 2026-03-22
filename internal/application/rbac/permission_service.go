package rbac

import (
	"context"
	"errors"
	"sort"
	"strings"

	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"

	"go.uber.org/zap"
)

// PermissionService 权限用例服务。
type PermissionService struct {
	roles     rbacdomain.RoleRepository
	roleMenus rbacdomain.RoleMenuRepository
	resources rbacdomain.ApiResourceRepository
	enforcer  Enforcer
	graph     *RoleGraph
	evaluator *rbacdomain.PermissionEvaluator
	inherit   rbacdomain.Inherit
	logger    *zap.Logger
}

// NewPermissionService 构造权限用例服务。
func NewPermissionService(
	roles rbacdomain.RoleRepository,
	roleMenus rbacdomain.RoleMenuRepository,
	resources rbacdomain.ApiResourceRepository,
	enforcer Enforcer,
	graph *RoleGraph,
	evaluator *rbacdomain.PermissionEvaluator,
	logger *zap.Logger,
) *PermissionService {
	return &PermissionService{
		roles:     roles,
		roleMenus: roleMenus,
		resources: resources,
		enforcer:  enforcer,
		graph:     graph,
		evaluator: evaluator,
		inherit:   rbacdomain.Inherit{},
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
	perms, err := s.evaluator.CollectPermissions(params.Permissions)
	if err != nil {
		return err
	}

	// 验证角色是否存在
	role, err := s.roles.FindByName(ctx, params.RoleName)
	if err != nil || role == nil {
		return rbacdomain.ErrRoleNotFound
	}

	// 列表内部冗余（如同时含 path * 与 path GET）
	if err := s.evaluator.CheckInternalRedundancy(perms); err != nil {
		return err
	}

	// 已有有效权限（直接 + 继承）覆盖则拒绝
	effective, err := s.graph.EffectivePermissionItems(params.RoleName)
	if err != nil {
		return rbacdomain.ErrQueryPermissionFailed
	}

	if err := s.evaluator.CheckAlreadyOwned(effective, perms); err != nil {
		return err
	}

	for _, p := range perms {
		if _, err := s.enforcer.AddPolicy(params.RoleName, p.Resource, p.Action); err != nil {
			return rbacdomain.ErrAddFailed
		}
	}

	if err := s.enforcer.SavePolicy(); err != nil {
		return rbacdomain.ErrSaveFailed
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
	perms, err := s.evaluator.CollectPermissions(params.Permissions)
	if err != nil {
		return err
	}

	// 验证角色是否存在
	role, err := s.roles.FindByName(ctx, params.RoleName)
	if err != nil || role == nil {
		return rbacdomain.ErrRoleNotFound
	}

	for _, p := range perms {
		if _, err := s.enforcer.RemovePolicy(params.RoleName, p.Resource, p.Action); err != nil {
			s.logger.Error("移除失败", zap.Error(err))

			return rbacdomain.ErrRemoveFailed
		}
	}

	if err := s.enforcer.SavePolicy(); err != nil {
		s.logger.Error("保存失败", zap.Error(err))

		return rbacdomain.ErrSaveFailed
	}

	return nil
}

// RolePermissions 获取角色权限（直接权限 + 有效权限 + 覆盖的具体 API）。
func (s *PermissionService) RolePermissions(ctx context.Context, roleName string) (RolePermissionsResp, error) {
	role, err := s.roles.FindByName(ctx, roleName)
	if err != nil || role == nil {
		return RolePermissionsResp{}, rbacdomain.ErrRoleNotFound
	}

	directRows, err := s.enforcer.GetPermissionsForUser(roleName)
	if err != nil {
		s.logger.Error("获取角色直接权限失败", zap.Error(err))

		return RolePermissionsResp{}, rbacdomain.ErrGetPermissionFailed
	}

	effectiveRows, err := s.enforcer.GetImplicitPermissionsForUser(roleName)
	if err != nil {
		s.logger.Error("获取角色有效权限失败", zap.Error(err))

		return RolePermissionsResp{}, rbacdomain.ErrGetPermissionFailed
	}

	directItems := s.evaluator.NormalizeCasbinPolicyRows(directRows)

	directParents, err := s.graph.DirectParents(roleName)
	if err != nil {
		s.logger.Error("获取直接父角色失败", zap.Error(err))

		return RolePermissionsResp{}, rbacdomain.ErrGetPermissionFailed
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
	effectiveItems := make([]rbacdomain.PermissionItem, 0, len(effectiveRows))

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
			top, kind, ferr := s.inherit.FindTopParentForOwner(s.graph, roleName, owner)
			if ferr != nil {
				s.logger.Warn("解析继承顶层父角色失败", zap.Error(ferr), zap.String("owner", owner))
			}

			if kind == rbacdomain.InheritKindDirect {
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
			effectiveItems = append(effectiveItems, rbacdomain.PermissionItem{Resource: resource, Action: action})
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
		s.logger.Error("读取资源表失败", zap.Error(err))

		return RolePermissionsResp{}, rbacdomain.ErrGetPermissionFailed
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
		return rbacdomain.ErrRoleNotFound
	}

	perms, err := s.evaluator.NormalizeReplacementPermissions(params.Permissions)
	if err != nil {
		return err
	}

	// 列表内部冗余
	if err := s.evaluator.CheckInternalRedundancy(perms); err != nil {
		return err
	}

	if params.BreakInheritance {
		previousMenus, err := s.roleMenus.InheritedMenuIDs(ctx, role.ID())
		if err != nil {
			return rbacdomain.ErrBreakInheritanceFailed
		}
		defer func() {
			if result == nil {
				return
			}
			ids := make([]uint, 0, len(previousMenus))
			for id := range previousMenus {
				ids = append(ids, id)
			}
			if err := s.roleMenus.CreateIgnoreConflict(context.WithoutCancel(ctx), rbacdomain.NewInheritedRoleMenus(role.ID(), ids)); err != nil {
				s.logger.Error("RBAC inherited menu compensation failed", zap.Error(err))
				result = errors.Join(result, err)
			}
		}()
		if err := s.clearRoleInheritance(ctx, role); err != nil {
			return err
		}
	} else {
		// 未解除继承时：新写入的直接权限不得已被继承权限覆盖
		direct, err := s.graph.DirectPermissionItems(role.Name())
		if err != nil {
			return rbacdomain.ErrQueryPermissionFailed
		}

		effective, err := s.graph.EffectivePermissionItems(role.Name())
		if err != nil {
			return rbacdomain.ErrQueryPermissionFailed
		}

		inherited := s.inherit.InheritedPermissions(direct, effective)
		if err := s.evaluator.CheckAlreadyOwned(inherited, perms); err != nil {
			return err
		}
	}

	// 清空原有直接权限
	if _, err := s.enforcer.RemoveFilteredPolicy(0, role.Name()); err != nil {
		s.logger.Error("清理角色权限失败", zap.Error(err))

		return rbacdomain.ErrUpdateFailed
	}

	for _, p := range perms {
		if _, err := s.enforcer.AddPolicy(role.Name(), p.Resource, p.Action); err != nil {
			s.logger.Error("添加角色权限失败", zap.Error(err))

			return rbacdomain.ErrUpdateFailed
		}
	}

	if err := s.enforcer.SavePolicy(); err != nil {
		s.logger.Error("保存角色权限失败", zap.Error(err))

		return rbacdomain.ErrUpdateFailed
	}

	return nil
}

// clearRoleInheritance 清除角色的全部 Casbin 继承关系与继承菜单。
func (s *PermissionService) clearRoleInheritance(ctx context.Context, role *rbacdomain.Role) error {
	parents, err := s.enforcer.GetRolesForUser(role.Name())
	if err != nil {
		s.logger.Error("读取角色继承失败", zap.Error(err))

		return rbacdomain.ErrBreakInheritanceFailed
	}

	for _, parent := range parents {
		if _, err := s.enforcer.DeleteRoleForUser(role.Name(), parent); err != nil {
			s.logger.Error("移除角色继承失败", zap.Error(err), zap.String("parent", parent))

			return rbacdomain.ErrBreakInheritanceFailed
		}
	}

	if err := s.roleMenus.DeleteInheritedByRole(ctx, role.ID()); err != nil {
		s.logger.Error("移除继承菜单失败", zap.Error(err))

		return rbacdomain.ErrBreakInheritanceFailed
	}

	return nil
}

// WildcardList 返回后端所有已配置的通配符权限列表（跨角色去重后按资源、方法排序）。
func (s *PermissionService) WildcardList() ([]WildcardPermissionItem, error) {
	policies, err := s.enforcer.GetPolicy()
	if err != nil {
		s.logger.Error("读取权限策略失败", zap.Error(err))

		return nil, rbacdomain.ErrGetWildcardFailed
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

		if !s.evaluator.IsWildcardPermission(resource, action) {
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
		s.logger.Error("读取资源表失败", zap.Error(err))

		return nil, rbacdomain.ErrGetResourceFailed
	}

	items := make([]APIResourceItem, 0, len(resources))

	for _, item := range resources {
		items = append(items, APIResourceItem{
			Path:    item.Path,
			Method:  item.Method,
			Summary: item.Summary,
			Tags:    item.Tags,
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
