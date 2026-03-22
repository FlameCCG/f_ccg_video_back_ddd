package rbac

import (
	"context"

	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"

	"go.uber.org/zap"
)

// groupingPolicyName Casbin 角色继承分组策略的名称。
const groupingPolicyName = "g"

// InheritService 角色继承用例服务。
type InheritService struct {
	roles     rbacdomain.RoleRepository
	roleMenus rbacdomain.RoleMenuRepository
	enforcer  Enforcer
	graph     *RoleGraph
	inherit   rbacdomain.Inherit
	logger    *zap.Logger
}

// NewInheritService 构造角色继承用例服务。
func NewInheritService(
	roles rbacdomain.RoleRepository,
	roleMenus rbacdomain.RoleMenuRepository,
	enforcer Enforcer,
	graph *RoleGraph,
	logger *zap.Logger,
) *InheritService {
	return &InheritService{roles: roles, roleMenus: roleMenus, enforcer: enforcer, graph: graph, inherit: rbacdomain.Inherit{}, logger: logger}
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
		return rbacdomain.ErrRoleNotFound
	}

	parent, err := s.roles.FindByID(ctx, parentRoleID)
	if err != nil || parent == nil {
		return rbacdomain.ErrParentRoleNotFound
	}

	if err := role.EnsureCanInherit(parent); err != nil {
		return err
	}

	// 已直接或间接继承则拒绝
	already, kind, topParent, err := s.inherit.IsAlreadyInherited(s.graph, role.Name(), parent.Name())
	if err != nil {
		s.logger.Error("检查角色继承关系失败", zap.Error(err))

		return rbacdomain.ErrInheritFailed
	}

	if already {
		return rbacdomain.AlreadyInheritedError(parent.Name(), kind, topParent)
	}

	// 防止循环继承：父角色已继承当前角色
	cycle, err := s.inherit.WouldCreateCycle(s.graph, role.Name(), parent.Name())
	if err != nil {
		s.logger.Error("检查循环继承失败", zap.Error(err))

		return rbacdomain.ErrInheritFailed
	}

	if cycle {
		return rbacdomain.ErrInheritCycle
	}

	if _, err := s.enforcer.AddRoleForUser(role.Name(), parent.Name()); err != nil {
		s.logger.Error("Casbin添加角色继承失败", zap.Error(err))

		return rbacdomain.ErrInheritFailed
	}

	if err := s.enforcer.SavePolicy(); err != nil {
		s.logger.Error("Casbin保存失败", zap.Error(err))

		return rbacdomain.ErrPermissionSaveFailed
	}

	parentMenuIDs, err := s.roleMenus.DistinctMenuIDsByRole(ctx, parent.ID())
	if err != nil {
		s.logger.Error("读取父角色菜单失败", zap.Error(err))

		return rbacdomain.ErrInheritFailed
	}

	if len(parentMenuIDs) == 0 {
		return nil
	}

	existingMenuIDs, err := s.roleMenus.MenuIDsByRole(ctx, role.ID())
	if err != nil {
		s.logger.Error("读取角色菜单失败", zap.Error(err))

		return rbacdomain.ErrInheritFailed
	}

	missing := s.inherit.MissingMenuIDs(parentMenuIDs, existingMenuIDs)
	if len(missing) == 0 {
		return nil
	}

	if err := s.roleMenus.CreateIgnoreConflict(ctx, rbacdomain.NewInheritedRoleMenus(role.ID(), missing)); err != nil {
		s.logger.Error("继承父角色菜单失败", zap.Error(err))

		return rbacdomain.ErrInheritFailed
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
		return rbacdomain.ErrRoleNotFound
	}

	parent, err := s.roles.FindByID(ctx, parentRoleID)
	if err != nil || parent == nil {
		return rbacdomain.ErrParentRoleNotFound
	}

	directParents, err := s.graph.DirectParents(role.Name())
	if err != nil {
		s.logger.Error("读取直接父角色失败", zap.Error(err))

		return rbacdomain.ErrRemoveFailed
	}

	if !s.inherit.ContainsRoleName(directParents, parent.Name()) {
		// 是否为间接继承
		already, kind, topParent, ierr := s.inherit.IsAlreadyInherited(s.graph, role.Name(), parent.Name())
		if ierr != nil {
			s.logger.Error("检查继承关系失败", zap.Error(ierr))

			return rbacdomain.ErrRemoveFailed
		}

		if already && kind == rbacdomain.InheritKindIndirect {
			return rbacdomain.IndirectUnlinkError(parent.Name(), topParent)
		}

		return rbacdomain.ErrNotDirectInherit
	}

	if _, err := s.enforcer.DeleteRoleForUser(role.Name(), parent.Name()); err != nil {
		s.logger.Error("Casbin移除角色继承失败", zap.Error(err))

		return rbacdomain.ErrRemoveFailed
	}

	if err := s.enforcer.SavePolicy(); err != nil {
		s.logger.Error("Casbin保存失败", zap.Error(err))

		return rbacdomain.ErrPermissionSaveFailed
	}

	parents, err := s.enforcer.GetRolesForUser(role.Name())
	if err != nil {
		s.logger.Error("读取角色继承失败", zap.Error(err))

		return rbacdomain.ErrRemoveFailed
	}

	if len(parents) == 0 {
		return s.dropInheritedMenus(ctx, role.ID())
	}

	parentRoles, err := s.roles.FindBriefByNames(ctx, parents)
	if err != nil {
		s.logger.Error("读取父角色失败", zap.Error(err))

		return rbacdomain.ErrRemoveFailed
	}

	parentIDs := make([]uint, 0, len(parentRoles))

	for _, p := range parentRoles {
		if p.ID() == 0 {
			continue
		}

		parentIDs = append(parentIDs, p.ID())
	}

	if len(parentIDs) == 0 {
		return s.dropInheritedMenus(ctx, role.ID())
	}

	keepMenuIDs, err := s.roleMenus.DistinctMenuIDsByRoles(ctx, parentIDs)
	if err != nil {
		s.logger.Error("读取父角色菜单失败", zap.Error(err))

		return rbacdomain.ErrRemoveFailed
	}

	if len(keepMenuIDs) == 0 {
		return s.dropInheritedMenus(ctx, role.ID())
	}

	if err := s.roleMenus.DeleteInheritedByRoleExcept(ctx, role.ID(), keepMenuIDs); err != nil {
		s.logger.Error("移除继承菜单失败", zap.Error(err))

		return rbacdomain.ErrRemoveFailed
	}

	return nil
}

// dropInheritedMenus 清空角色的全部继承菜单。
func (s *InheritService) dropInheritedMenus(ctx context.Context, roleID uint) error {
	if err := s.roleMenus.DeleteInheritedByRole(ctx, roleID); err != nil {
		s.logger.Error("移除继承菜单失败", zap.Error(err))

		return rbacdomain.ErrRemoveFailed
	}

	return nil
}

// List 获取角色继承列表（直接 + 间接）。
func (s *InheritService) List(ctx context.Context, roleID uint) (RoleInheritsResp, error) {
	role, err := s.roles.FindByID(ctx, roleID)
	if err != nil || role == nil {
		return RoleInheritsResp{}, rbacdomain.ErrRoleNotFound
	}
	direct, err := s.graph.DirectParents(role.Name())
	if err != nil {
		s.logger.Error("查询直接父角色失败", zap.Error(err))
		return RoleInheritsResp{}, rbacdomain.ErrQueryFailed
	}
	implicit, err := s.graph.ImplicitParents(role.Name())
	if err != nil {
		s.logger.Error("查询隐式父角色失败", zap.Error(err))
		return RoleInheritsResp{}, rbacdomain.ErrQueryFailed
	}

	selection := s.inherit.SelectParents(role.Name(), direct, implicit)
	indirect := make([]RoleIndirectInheritItem, 0, len(selection.Indirect))
	for _, name := range selection.Indirect {
		top, _, err := s.inherit.FindTopParentForOwner(s.graph, role.Name(), name)
		if err != nil {
			s.logger.Error("解析间接继承顶层父角色失败", zap.Error(err), zap.String("owner", name))
			// 保留原降级语义：查询失败仍返回该项，via 为空。
			indirect = append(indirect, RoleIndirectInheritItem{Name: name})
			continue
		}
		indirect = append(indirect, RoleIndirectInheritItem{Name: name, Via: top})
	}
	return RoleInheritsResp{Direct: selection.Direct, Indirect: indirect}, nil
}

// Tree 构建全量角色继承树。
func (s *InheritService) Tree(ctx context.Context) ([]*rbacdomain.RoleInheritTreeNode, error) {
	if s.enforcer == nil {
		return nil, rbacdomain.ErrEnforcerNotInitialized
	}

	roles, err := s.roles.ListBrief(ctx)
	if err != nil {
		return nil, rbacdomain.ErrQueryRoleFailed
	}

	if len(roles) == 0 {
		return []*rbacdomain.RoleInheritTreeNode{}, nil
	}

	policies, err := s.enforcer.GetNamedGroupingPolicy(groupingPolicyName)
	if err != nil {
		return nil, rbacdomain.ErrQueryFailed
	}

	return s.inherit.BuildTree(roles, policies), nil
}

// FilterTree 只保留与指定角色相关的继承子树。
func (s *InheritService) FilterTree(
	ctx context.Context,
	tree []*rbacdomain.RoleInheritTreeNode,
	roleID uint,
) ([]*rbacdomain.RoleInheritTreeNode, error) {
	role, err := s.roles.FindBriefByID(ctx, roleID)
	if err != nil || role == nil {
		return nil, rbacdomain.ErrRoleNotFound
	}

	roleName, err := role.CheckedName()
	if err != nil {
		return nil, err
	}

	if len(tree) == 0 {
		return []*rbacdomain.RoleInheritTreeNode{}, nil
	}

	return s.inherit.FilterTree(tree, roleName), nil
}
