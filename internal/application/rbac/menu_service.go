package rbac

import (
	"context"
	rbacservice "f_ccg_video_back_ddd/internal/domain/rbac/service"
	"fmt"
	"strings"

	rbacerrors "f_ccg_video_back_ddd/internal/domain/rbac/domainerr"
	rbacentity "f_ccg_video_back_ddd/internal/domain/rbac/entity"
	rbacrepo "f_ccg_video_back_ddd/internal/domain/rbac/repository"
	rbacvo "f_ccg_video_back_ddd/internal/domain/rbac/valueobject"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"

	"go.uber.org/zap"
)

const (
	// MenuMoveBefore 将菜单移动到目标菜单之前。
	MenuMoveBefore = rbacservice.MenuMoveBefore
	// MenuMoveAfter 将菜单移动到目标菜单之后。
	MenuMoveAfter = rbacservice.MenuMoveAfter
)

// MenuService 菜单用例服务。
type MenuService struct {
	txm       tx.Manager
	roles     rbacrepo.RoleRepository
	menus     rbacrepo.MenuRepository
	roleMenus rbacrepo.RoleMenuRepository
	userRoles rbacrepo.UserRoleRepository
	generator MenuSuggestionGenerator
	logger    *zap.Logger
}

// NewMenuService 构造菜单用例服务。
func NewMenuService(
	txm tx.Manager,
	roles rbacrepo.RoleRepository,
	menus rbacrepo.MenuRepository,
	roleMenus rbacrepo.RoleMenuRepository,
	userRoles rbacrepo.UserRoleRepository,
	generator MenuSuggestionGenerator,
	logger *zap.Logger,
) *MenuService {
	return &MenuService{
		txm:       txm,
		roles:     roles,
		menus:     menus,
		roleMenus: roleMenus,
		userRoles: userRoles,
		generator: generator,
		logger:    logger,
	}
}

// Create 原子创建菜单、解决同层排序冲突，并把菜单直连给创建者当前角色。
func (s *MenuService) Create(ctx context.Context, params CreateMenuParams) error {
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return rbacerrors.ErrMenuTitleEmpty
	}

	icon := strings.TrimSpace(params.Icon)
	if err := rbacvo.ValidateMenuIcon(icon); err != nil {
		return err
	}

	menu := rbacentity.NewMenu(
		title,
		strings.TrimSpace(params.TitleEn),
		strings.TrimSpace(params.TitleJa),
		icon,
		strings.TrimSpace(params.Path),
		strings.TrimSpace(params.Name),
		strings.TrimSpace(params.Component),
		params.KeepAlive,
		normalizedParentID(params.ParentID),
		params.SortOrder,
	)

	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		if err := s.ensureMenuTitleAvailable(txCtx, menu.Title, 0); err != nil {
			return err
		}

		if err := s.validateMenuParent(txCtx, 0, menu.ParentID); err != nil {
			return err
		}

		siblingIDs, err := s.menus.ListSiblingIDsForUpdate(txCtx, menu.ParentID)
		if err != nil {
			return fmt.Errorf("读取同层菜单失败: %w", err)
		}

		plan := rbacservice.PlanMenuInsertion(menu.ID, menu.SortOrder, siblingIDs)
		siblingIDs = plan.Sequence
		if err := s.applyMenuSequence(txCtx, siblingIDs); err != nil {
			return err
		}

		menu.SortOrder = plan.SortOrder
		if err := s.menus.Create(txCtx, menu); err != nil {
			return fmt.Errorf("创建菜单失败: %w", err)
		}

		return s.grantMenuToCreatorRole(txCtx, params.CreatorID, menu.ID)
	})

	return err
}

// Update 更新菜单字段；父级或排序变化时会原子地重排新旧层级。
func (s *MenuService) Update(ctx context.Context, params UpdateMenuParams) error {
	if params.Title == nil && params.TitleEn == nil && params.TitleJa == nil && params.Icon == nil &&
		params.Path == nil && params.Name == nil && params.Component == nil && params.KeepAlive == nil &&
		!params.ParentProvided && params.SortOrder == nil {
		return rbacerrors.ErrNoUpdatableFields
	}

	update, err := menuFieldUpdate(params)
	if err != nil {
		return err
	}

	err = s.txm.Do(ctx, func(txCtx context.Context) error {
		menu, err := s.menus.FindByIDForUpdate(txCtx, params.ID)
		if err != nil {
			return fmt.Errorf("读取菜单失败: %w", err)
		}
		if menu == nil {
			return rbacerrors.ErrMenuNotFound
		}

		if update.Title != nil {
			if err := s.ensureMenuTitleAvailable(txCtx, *update.Title, menu.ID); err != nil {
				return err
			}
		}

		nextParentID := normalizedParentID(menu.ParentID)
		if params.ParentProvided {
			nextParentID = normalizedParentID(params.ParentID)
		}

		if err := s.validateMenuParent(txCtx, menu.ID, nextParentID); err != nil {
			return err
		}

		if params.ParentProvided || params.SortOrder != nil {
			if err := s.placeExistingMenu(txCtx, menu, nextParentID, params.SortOrder); err != nil {
				return err
			}
		}

		if update.IsEmpty() {
			return nil
		}

		update, err = menu.Revise(update)
		if err != nil {
			return err
		}
		if err := s.menus.UpdateFields(txCtx, menu.ID, update); err != nil {
			return fmt.Errorf("更新菜单失败: %w", err)
		}

		return nil
	})

	return err
}

// Move 把菜单移动到同层目标菜单的前面或后面，并重建该层连续排序。
func (s *MenuService) Move(ctx context.Context, menuID, targetID uint, position string) error {
	if menuID == targetID {
		return nil
	}

	if position != MenuMoveBefore && position != MenuMoveAfter {
		return rbacerrors.ErrMenuMovePosition
	}

	err := s.txm.Do(ctx, func(txCtx context.Context) error {
		menus, err := s.menus.FindByIDsForUpdate(txCtx, []uint{menuID, targetID})
		if err != nil {
			return fmt.Errorf("读取拖拽菜单失败: %w", err)
		}
		if len(menus) != 2 {
			return rbacerrors.ErrMenuNotFound
		}

		var moving, target *rbacentity.Menu
		for _, menu := range menus {
			if menu == nil {
				continue
			}

			switch menu.ID {
			case menuID:
				moving = menu
			case targetID:
				target = menu
			}
		}

		if moving == nil || target == nil {
			return rbacerrors.ErrMenuNotFound
		}

		parentID := normalizedParentID(moving.ParentID)
		if !rbacservice.SameMenuParent(parentID, target.ParentID) {
			return rbacerrors.ErrMenuMoveAcrossLevels
		}
		siblingIDs, err := s.menus.ListSiblingIDsForUpdate(txCtx, parentID)
		if err != nil {
			return fmt.Errorf("读取同层菜单失败: %w", err)
		}
		siblingIDs, err = rbacservice.PlanMenuMove(moving, target, siblingIDs, position)
		if err != nil {
			return err
		}

		return s.applyMenuSequence(txCtx, siblingIDs)
	})

	return err
}

func menuFieldUpdate(params UpdateMenuParams) (rbacentity.MenuFieldUpdate, error) {
	return (rbacentity.MenuFieldUpdate{
		Title: params.Title, TitleEn: params.TitleEn, TitleJa: params.TitleJa,
		Icon: params.Icon, Path: params.Path, Name: params.Name, Component: params.Component,
		KeepAlive: params.KeepAlive,
	}).Normalize()
}

func (s *MenuService) placeExistingMenu(
	ctx context.Context,
	menu *rbacentity.Menu,
	nextParentID *uint,
	sortOrder *uint,
) error {
	oldParentID := normalizedParentID(menu.ParentID)
	oldSiblingIDs, err := s.menus.ListSiblingIDsForUpdate(ctx, oldParentID)
	if err != nil {
		return fmt.Errorf("读取原层级菜单失败: %w", err)
	}

	var destinationIDs []uint
	if !rbacservice.SameMenuParent(oldParentID, nextParentID) {
		destinationIDs, err = s.menus.ListSiblingIDsForUpdate(ctx, nextParentID)
		if err != nil {
			return fmt.Errorf("读取目标层级菜单失败: %w", err)
		}
	}
	plan := rbacservice.PlanMenuPlacement(menu, nextParentID, sortOrder, oldSiblingIDs, destinationIDs)
	if plan.ParentChanged {
		if err := s.applyMenuSequence(ctx, plan.SourceOrder); err != nil {
			return err
		}
	}
	if err := s.applyMenuSequence(ctx, plan.DestinationOrder); err != nil {
		return err
	}
	if !plan.ParentChanged {
		return nil
	}

	parentUpdate := rbacentity.MenuFieldUpdate{}
	if nextParentID == nil {
		parentUpdate.ClearParent = true
	} else {
		parentUpdate.ParentID = nextParentID
	}

	if err := s.menus.UpdateFields(ctx, menu.ID, parentUpdate); err != nil {
		return fmt.Errorf("更新父菜单失败: %w", err)
	}

	return nil
}

func (s *MenuService) applyMenuSequence(ctx context.Context, menuIDs []uint) error {
	for index, menuID := range menuIDs {
		if menuID == 0 {
			continue
		}

		if err := s.menus.UpdateSortOrder(ctx, menuID, uint(index+1)); err != nil {
			return fmt.Errorf("更新菜单排序失败: %w", err)
		}
	}

	return nil
}

func (s *MenuService) grantMenuToCreatorRole(ctx context.Context, creatorID, menuID uint) error {
	if creatorID == 0 {
		return rbacerrors.ErrMenuCreatorRoleMissing
	}

	roleID, found, err := s.userRoles.CurrentRoleID(ctx, creatorID)
	if err != nil {
		return fmt.Errorf("读取创建者角色失败: %w", err)
	}
	if !found || roleID == 0 {
		return rbacerrors.ErrMenuCreatorRoleMissing
	}

	role, err := s.roles.FindIDByID(ctx, roleID)
	if err != nil {
		return fmt.Errorf("读取创建者当前角色失败: %w", err)
	}
	if role == nil {
		return rbacerrors.ErrMenuCreatorRoleMissing
	}

	if err := s.roleMenus.GrantDirect(ctx, role.ID, menuID); err != nil {
		return fmt.Errorf("授予创建者菜单权限失败: %w", err)
	}

	return nil
}

func (s *MenuService) ensureMenuTitleAvailable(ctx context.Context, title string, excludeID uint) error {
	count, err := s.menus.CountByTitleExcluding(ctx, title, excludeID)
	if err != nil {
		return fmt.Errorf("检查菜单标题失败: %w", err)
	}
	if count > 0 {
		return rbacerrors.ErrMenuExists
	}

	return nil
}

func (s *MenuService) validateMenuParent(ctx context.Context, menuID uint, parentID *uint) error {
	if parentID == nil {
		return nil
	}

	visited := make(map[uint]struct{})
	currentID := *parentID
	for currentID > 0 {
		if menuID > 0 && currentID == menuID {
			return rbacerrors.ErrMenuParentCycle
		}
		if _, ok := visited[currentID]; ok {
			return rbacerrors.ErrMenuParentCycle
		}
		visited[currentID] = struct{}{}

		parent, err := s.menus.FindByIDForUpdate(ctx, currentID)
		if err != nil {
			return fmt.Errorf("读取父菜单失败: %w", err)
		}
		if parent == nil {
			return rbacerrors.ErrParentMenuNotFound
		}

		next := normalizedParentID(parent.ParentID)
		if next == nil {
			return nil
		}

		currentID = *next
	}

	return nil
}

func normalizedParentID(parentID *uint) *uint {
	if parentID == nil || *parentID == 0 {
		return nil
	}

	value := *parentID

	return &value
}

// Delete 删除菜单：存在子菜单时拒绝，删除前先清理角色-菜单关联。
func (s *MenuService) Delete(ctx context.Context, menuID uint) error {
	menu, err := s.menus.FindByID(ctx, menuID)
	if err != nil || menu == nil {
		return rbacerrors.ErrMenuNotFound
	}

	childCount, err := s.menus.CountChildren(ctx, menu.ID)
	if err != nil {
		return rbacerrors.ErrQueryFailed
	}

	if err := menu.EnsureDeletable(childCount); err != nil {
		return err
	}

	if err := s.roleMenus.DeleteByMenu(ctx, menu.ID); err != nil {
		return rbacerrors.ErrDeleteFailed
	}

	if err := s.menus.DeleteByID(ctx, menu.ID); err != nil {
		return rbacerrors.ErrDeleteFailed
	}

	return nil
}

// AssignRoleMenus 用给定菜单集合整体替换角色的直连菜单（继承菜单不受影响）。
func (s *MenuService) AssignRoleMenus(ctx context.Context, roleID uint, menuIDs []uint) error {
	role, err := s.roles.FindIDByID(ctx, roleID)
	if err != nil || role == nil {
		return rbacerrors.ErrRoleNotFound
	}

	inheritMenuIDs, err := s.roleMenus.InheritedMenuIDs(ctx, role.ID)
	if err != nil {
		return rbacerrors.ErrQueryFailed
	}

	filteredIDs := rbacentity.DedupeMenuIDs(menuIDs, inheritMenuIDs)
	if len(filteredIDs) == 0 {
		if err := s.roleMenus.DeleteDirectByRole(ctx, role.ID); err != nil {
			s.logger.Error("清空菜单失败", zap.Error(err))

			return rbacerrors.ErrUpdateFailed
		}

		return nil
	}

	menus, err := s.menus.FindByIDs(ctx, filteredIDs)
	if err != nil {
		s.logger.Error("查询菜单失败", zap.Error(err))

		return rbacerrors.ErrQueryMenuFailed
	}
	if len(menus) != len(filteredIDs) {
		return rbacerrors.ErrSomeMenusNotFound
	}

	if err := s.roleMenus.DeleteDirectByRoleExcept(ctx, role.ID, filteredIDs); err != nil {
		s.logger.Error("更新菜单失败", zap.Error(err))

		return rbacerrors.ErrUpdateFailed
	}

	if err := s.roleMenus.CreateIgnoreConflict(ctx, rbacentity.NewDirectRoleMenus(role.ID, menus)); err != nil {
		s.logger.Error("更新菜单失败", zap.Error(err))

		return rbacerrors.ErrUpdateFailed
	}

	return nil
}

// RemoveRoleMenus 移除角色的指定直连菜单（继承菜单不可移除）。
func (s *MenuService) RemoveRoleMenus(ctx context.Context, roleID uint, menuIDs []uint) error {
	role, err := s.roles.FindIDByID(ctx, roleID)
	if err != nil || role == nil {
		return rbacerrors.ErrRoleNotFound
	}

	inheritMenuIDs, err := s.roleMenus.InheritedMenuIDs(ctx, role.ID)
	if err != nil {
		return rbacerrors.ErrQueryFailed
	}

	filteredIDs := rbacentity.DedupeMenuIDs(menuIDs, inheritMenuIDs)
	if len(filteredIDs) == 0 {
		return nil
	}

	menus, err := s.menus.FindByIDs(ctx, filteredIDs)
	if err != nil {
		return rbacerrors.ErrQueryMenuFailed
	}
	if len(menus) != len(filteredIDs) {
		return rbacerrors.ErrSomeMenusNotFound
	}

	if err := s.roleMenus.DeleteDirectByRoleAndMenus(ctx, role.ID, filteredIDs); err != nil {
		s.logger.Error("移除菜单失败", zap.Error(err))

		return rbacerrors.ErrRemoveFailed
	}

	return nil
}
