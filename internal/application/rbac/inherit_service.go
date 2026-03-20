package rbac

import (
	"context"
	"strings"

	rbacerrors "f_ccg_video_back_ddd/internal/domain/rbac/domainerr"
	rbacentity "f_ccg_video_back_ddd/internal/domain/rbac/entity"
	rbacrepo "f_ccg_video_back_ddd/internal/domain/rbac/repository"
	rbacservice "f_ccg_video_back_ddd/internal/domain/rbac/service"

	"go.uber.org/zap"
)

// groupingPolicyName Casbin 角色继承分组策略的名称。
const groupingPolicyName = "g"

// InheritService 角色继承用例服务。
type InheritService struct {
	roles     rbacrepo.RoleRepository
	roleMenus rbacrepo.RoleMenuRepository
	enforcer  Enforcer
	graph     *RoleGraph
	logger    *zap.Logger
}

// NewInheritService 构造角色继承用例服务。
func NewInheritService(
	roles rbacrepo.RoleRepository,
	roleMenus rbacrepo.RoleMenuRepository,
	enforcer Enforcer,
	graph *RoleGraph,
	logger *zap.Logger,
) *InheritService {
	return &InheritService{roles: roles, roleMenus: roleMenus, enforcer: enforcer, graph: graph, logger: logger}
}

// Assign 建立角色继承关系，并把父角色菜单以「继承」标记同步给子角色。
func (s *InheritService) Assign(ctx context.Context, roleID, parentRoleID uint) (result error) {
	scoped, finish, err := beginPolicyChange(s.enforcer)
	if err != nil {
		return err
	}
	defer finish(&result)
	service := *s
	service.enforcer = scoped
	s = &service
	s.graph = NewRoleGraph(scoped)
	role, err := s.roles.FindByID(ctx, roleID)
	if err != nil || role == nil {
		return rbacerrors.ErrRoleNotFound
	}

	parent, err := s.roles.FindByID(ctx, parentRoleID)
	if err != nil || parent == nil {
		return rbacerrors.ErrParentRoleNotFound
	}

	if role.ID == parent.ID {
		return rbacerrors.ErrParentRoleIsSelf
	}

	// 已直接或间接继承则拒绝
	already, kind, topParent, err := s.graph.IsAlreadyInherited(role.Name, parent.Name)
	if err != nil {
		s.logger.Error("检查角色继承关系失败", zap.Error(err))

		return rbacerrors.ErrInheritFailed
	}

	if already {
		return rbacservice.AlreadyInheritedError(parent.Name, kind, topParent)
	}

	// 防止循环继承：父角色已继承当前角色
	cycle, err := s.graph.WouldCreateCycle(role.Name, parent.Name)
	if err != nil {
		s.logger.Error("检查循环继承失败", zap.Error(err))

		return rbacerrors.ErrInheritFailed
	}

	if cycle {
		return rbacerrors.ErrInheritCycle
	}

	if _, err := s.enforcer.AddRoleForUser(role.Name, parent.Name); err != nil {
		s.logger.Error("Casbin添加角色继承失败", zap.Error(err))

		return rbacerrors.ErrInheritFailed
	}

	if err := s.enforcer.SavePolicy(); err != nil {
		s.logger.Error("Casbin保存失败", zap.Error(err))

		return rbacerrors.ErrPermissionSaveFailed
	}

	parentMenuIDs, err := s.roleMenus.DistinctMenuIDsByRole(ctx, parent.ID)
	if err != nil {
		s.logger.Error("读取父角色菜单失败", zap.Error(err))

		return rbacerrors.ErrInheritFailed
	}

	if len(parentMenuIDs) == 0 {
		return nil
	}

	existingMenuIDs, err := s.roleMenus.MenuIDsByRole(ctx, role.ID)
	if err != nil {
		s.logger.Error("读取角色菜单失败", zap.Error(err))

		return rbacerrors.ErrInheritFailed
	}

	existing := make(map[uint]struct{}, len(existingMenuIDs))
	for _, id := range existingMenuIDs {
		existing[id] = struct{}{}
	}

	missing := make([]uint, 0)

	for _, menuID := range parentMenuIDs {
		if _, ok := existing[menuID]; ok {
			continue
		}

		missing = append(missing, menuID)
	}

	if len(missing) == 0 {
		return nil
	}

	if err := s.roleMenus.CreateIgnoreConflict(ctx, rbacentity.NewInheritedRoleMenus(role.ID, missing)); err != nil {
		s.logger.Error("继承父角色菜单失败", zap.Error(err))

		return rbacerrors.ErrInheritFailed
	}

	return nil
}

// Remove 解除角色的直接继承关系，并回收不再由任何父角色提供的继承菜单。
//
// 仅允许解除「直接继承」；间接继承须先解顶层父角色。
func (s *InheritService) Remove(ctx context.Context, roleID, parentRoleID uint) (result error) {
	scoped, finish, err := beginPolicyChange(s.enforcer)
	if err != nil {
		return err
	}
	defer finish(&result)
	service := *s
	service.enforcer = scoped
	s = &service
	s.graph = NewRoleGraph(scoped)
	role, err := s.roles.FindByID(ctx, roleID)
	if err != nil || role == nil {
		return rbacerrors.ErrRoleNotFound
	}

	parent, err := s.roles.FindByID(ctx, parentRoleID)
	if err != nil || parent == nil {
		return rbacerrors.ErrParentRoleNotFound
	}

	directParents, err := s.graph.DirectParents(role.Name)
	if err != nil {
		s.logger.Error("读取直接父角色失败", zap.Error(err))

		return rbacerrors.ErrRemoveFailed
	}

	if !rbacservice.ContainsRoleName(directParents, parent.Name) {
		// 是否为间接继承
		already, kind, topParent, ierr := s.graph.IsAlreadyInherited(role.Name, parent.Name)
		if ierr != nil {
			s.logger.Error("检查继承关系失败", zap.Error(ierr))

			return rbacerrors.ErrRemoveFailed
		}

		if already && kind == rbacservice.InheritKindIndirect {
			return rbacservice.IndirectUnlinkError(parent.Name, topParent)
		}

		return rbacerrors.ErrNotDirectInherit
	}

	if _, err := s.enforcer.DeleteRoleForUser(role.Name, parent.Name); err != nil {
		s.logger.Error("Casbin移除角色继承失败", zap.Error(err))

		return rbacerrors.ErrRemoveFailed
	}

	if err := s.enforcer.SavePolicy(); err != nil {
		s.logger.Error("Casbin保存失败", zap.Error(err))

		return rbacerrors.ErrPermissionSaveFailed
	}

	parents, err := s.enforcer.GetRolesForUser(role.Name)
	if err != nil {
		s.logger.Error("读取角色继承失败", zap.Error(err))

		return rbacerrors.ErrRemoveFailed
	}

	if len(parents) == 0 {
		return s.dropInheritedMenus(ctx, role.ID)
	}

	parentRoles, err := s.roles.FindBriefByNames(ctx, parents)
	if err != nil {
		s.logger.Error("读取父角色失败", zap.Error(err))

		return rbacerrors.ErrRemoveFailed
	}

	parentIDs := make([]uint, 0, len(parentRoles))

	for _, p := range parentRoles {
		if p.ID == 0 {
			continue
		}

		parentIDs = append(parentIDs, p.ID)
	}

	if len(parentIDs) == 0 {
		return s.dropInheritedMenus(ctx, role.ID)
	}

	keepMenuIDs, err := s.roleMenus.DistinctMenuIDsByRoles(ctx, parentIDs)
	if err != nil {
		s.logger.Error("读取父角色菜单失败", zap.Error(err))

		return rbacerrors.ErrRemoveFailed
	}

	if len(keepMenuIDs) == 0 {
		return s.dropInheritedMenus(ctx, role.ID)
	}

	if err := s.roleMenus.DeleteInheritedByRoleExcept(ctx, role.ID, keepMenuIDs); err != nil {
		s.logger.Error("移除继承菜单失败", zap.Error(err))

		return rbacerrors.ErrRemoveFailed
	}

	return nil
}

// dropInheritedMenus 清空角色的全部继承菜单。
func (s *InheritService) dropInheritedMenus(ctx context.Context, roleID uint) error {
	if err := s.roleMenus.DeleteInheritedByRole(ctx, roleID); err != nil {
		s.logger.Error("移除继承菜单失败", zap.Error(err))

		return rbacerrors.ErrRemoveFailed
	}

	return nil
}

// List 获取角色继承列表（直接 + 间接）。
func (s *InheritService) List(ctx context.Context, roleID uint) (RoleInheritsResp, error) {
	role, err := s.roles.FindByID(ctx, roleID)
	if err != nil || role == nil {
		return RoleInheritsResp{}, rbacerrors.ErrRoleNotFound
	}

	direct, err := s.graph.DirectParents(role.Name)
	if err != nil {
		s.logger.Error("查询直接父角色失败", zap.Error(err))

		return RoleInheritsResp{}, rbacerrors.ErrQueryFailed
	}

	implicit, err := s.graph.ImplicitParents(role.Name)
	if err != nil {
		s.logger.Error("查询隐式父角色失败", zap.Error(err))

		return RoleInheritsResp{}, rbacerrors.ErrQueryFailed
	}

	directSet := make(map[string]struct{}, len(direct))
	directList := make([]string, 0, len(direct))

	for _, name := range direct {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		if _, ok := directSet[name]; ok {
			continue
		}

		directSet[name] = struct{}{}
		directList = append(directList, name)
	}

	indirect := make([]RoleIndirectInheritItem, 0)
	seenIndirect := make(map[string]struct{})

	for _, name := range implicit {
		name = strings.TrimSpace(name)
		if name == "" || name == role.Name {
			continue
		}

		if _, ok := directSet[name]; ok {
			continue
		}

		if _, ok := seenIndirect[name]; ok {
			continue
		}

		seenIndirect[name] = struct{}{}

		top, _, ferr := s.graph.FindTopParentForOwner(role.Name, name)
		if ferr != nil {
			s.logger.Error("解析间接继承顶层父角色失败", zap.Error(ferr), zap.String("owner", name))
			// 仍返回该项，via 为空
			indirect = append(indirect, RoleIndirectInheritItem{Name: name})

			continue
		}

		indirect = append(indirect, RoleIndirectInheritItem{
			Name: name,
			Via:  top,
		})
	}

	return RoleInheritsResp{
		Direct:   directList,
		Indirect: indirect,
	}, nil
}

// Tree 构建全量角色继承树。
func (s *InheritService) Tree(ctx context.Context) ([]*rbacservice.RoleInheritTreeNode, error) {
	if s.enforcer == nil {
		return nil, rbacerrors.ErrEnforcerNotInitialized
	}

	roles, err := s.roles.ListBrief(ctx)
	if err != nil {
		return nil, rbacerrors.ErrQueryRoleFailed
	}

	if len(roles) == 0 {
		return []*rbacservice.RoleInheritTreeNode{}, nil
	}

	policies, err := s.enforcer.GetNamedGroupingPolicy(groupingPolicyName)
	if err != nil {
		return nil, rbacerrors.ErrQueryFailed
	}

	return rbacservice.BuildRoleInheritTree(roles, policies), nil
}

// FilterTree 只保留与指定角色相关的继承子树。
func (s *InheritService) FilterTree(
	ctx context.Context,
	tree []*rbacservice.RoleInheritTreeNode,
	roleID uint,
) ([]*rbacservice.RoleInheritTreeNode, error) {
	role, err := s.roles.FindBriefByID(ctx, roleID)
	if err != nil || role == nil {
		return nil, rbacerrors.ErrRoleNotFound
	}

	roleName := strings.TrimSpace(role.Name)
	if roleName == "" {
		return nil, rbacerrors.ErrRoleNameBlank
	}

	if len(tree) == 0 {
		return []*rbacservice.RoleInheritTreeNode{}, nil
	}

	return rbacservice.FilterRoleInheritTree(tree, roleName), nil
}
