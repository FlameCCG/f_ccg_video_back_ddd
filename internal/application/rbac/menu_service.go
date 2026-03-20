package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	rbacerrors "f_ccg_video_back_ddd/internal/domain/rbac/domainerr"
	rbacentity "f_ccg_video_back_ddd/internal/domain/rbac/entity"
	rbacrepo "f_ccg_video_back_ddd/internal/domain/rbac/repository"
	rbacvo "f_ccg_video_back_ddd/internal/domain/rbac/valueobject"
	"f_ccg_video_back_ddd/internal/domain/shared/tx"

	"go.uber.org/zap"
)

const (
	// MenuMoveBefore 将菜单移动到目标菜单之前。
	MenuMoveBefore = "before"
	// MenuMoveAfter 将菜单移动到目标菜单之后。
	MenuMoveAfter = "after"
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

// Tree 读取全部菜单并组装成树结构。
func (s *MenuService) Tree(ctx context.Context) ([]*rbacentity.Menu, error) {
	menus, err := s.menus.ListOrdered(ctx)
	if err != nil {
		return nil, rbacerrors.ErrGetFailed
	}

	return rbacentity.BuildMenuTree(menus), nil
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

		target := normalizedTargetSort(menu.SortOrder, len(siblingIDs))
		siblingIDs = insertMenuID(siblingIDs, menu.ID, int(target-1))
		if err := s.applyMenuSequence(txCtx, siblingIDs); err != nil {
			return err
		}

		menu.SortOrder = target
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
		if !sameParent(parentID, normalizedParentID(target.ParentID)) {
			return rbacerrors.ErrMenuMoveAcrossLevels
		}

		siblingIDs, err := s.menus.ListSiblingIDsForUpdate(txCtx, parentID)
		if err != nil {
			return fmt.Errorf("读取同层菜单失败: %w", err)
		}

		siblingIDs = removeMenuID(siblingIDs, menuID)
		targetIndex := menuIDIndex(siblingIDs, targetID)
		if targetIndex < 0 {
			return rbacerrors.ErrMenuNotFound
		}

		if position == MenuMoveAfter {
			targetIndex++
		}

		siblingIDs = insertMenuID(siblingIDs, menuID, targetIndex)

		return s.applyMenuSequence(txCtx, siblingIDs)
	})

	return err
}

// Generate 根据中文标题、父菜单和完整 menus 数据生成其余可编辑字段。
func (s *MenuService) Generate(
	ctx context.Context,
	title string,
	parentID *uint,
) (*rbacvo.MenuSuggestion, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, rbacerrors.ErrMenuGenerationTitleEmpty
	}

	parentID = normalizedParentID(parentID)
	if parentID != nil {
		parent, err := s.menus.FindByID(ctx, *parentID)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", rbacerrors.ErrReadParentMenuFailed, err)
		}
		if parent == nil {
			return nil, rbacerrors.ErrParentMenuNotFound
		}
	}

	menus, err := s.menus.ListForGeneration(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", rbacerrors.ErrReadMenuDataFailed, err)
	}

	menuTableJSON, err := json.Marshal(toMenuPromptRows(menus))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", rbacerrors.ErrPrepareMenuDataFailed, err)
	}

	if s.generator == nil {
		return nil, rbacerrors.ErrMenuGeneratorUnavailable
	}

	text, err := s.generator.GenerateMenuSuggestion(ctx, title, parentID, menuTableJSON)
	if err != nil {
		return nil, err
	}
	return parseMenuSuggestion(text)
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

	oldPosition := menuIDIndex(oldSiblingIDs, menu.ID) + 1
	oldSiblingIDs = removeMenuID(oldSiblingIDs, menu.ID)

	var destinationIDs []uint
	if sameParent(oldParentID, nextParentID) {
		destinationIDs = oldSiblingIDs
	} else {
		if err := s.applyMenuSequence(ctx, oldSiblingIDs); err != nil {
			return err
		}

		destinationIDs, err = s.menus.ListSiblingIDsForUpdate(ctx, nextParentID)
		if err != nil {
			return fmt.Errorf("读取目标层级菜单失败: %w", err)
		}
	}

	var requestedSort uint
	if sortOrder != nil {
		requestedSort = *sortOrder
	} else if sameParent(oldParentID, nextParentID) && oldPosition > 0 {
		requestedSort = uint(oldPosition)
	}

	target := normalizedTargetSort(requestedSort, len(destinationIDs))
	destinationIDs = insertMenuID(destinationIDs, menu.ID, int(target-1))
	if err := s.applyMenuSequence(ctx, destinationIDs); err != nil {
		return err
	}

	if sameParent(oldParentID, nextParentID) {
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

func normalizedTargetSort(requested uint, siblingCount int) uint {
	maximum := uint(siblingCount + 1)
	if requested == 0 || requested > maximum {
		return maximum
	}

	return requested
}

func sameParent(left, right *uint) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}

	return *left == *right
}

func removeMenuID(menuIDs []uint, menuID uint) []uint {
	result := make([]uint, 0, len(menuIDs))
	for _, id := range menuIDs {
		if id != menuID {
			result = append(result, id)
		}
	}

	return result
}

func insertMenuID(menuIDs []uint, menuID uint, index int) []uint {
	if index < 0 {
		index = 0
	}
	if index > len(menuIDs) {
		index = len(menuIDs)
	}

	result := make([]uint, 0, len(menuIDs)+1)
	result = append(result, menuIDs[:index]...)
	result = append(result, menuID)
	result = append(result, menuIDs[index:]...)

	return result
}

func menuIDIndex(menuIDs []uint, menuID uint) int {
	for index, id := range menuIDs {
		if id == menuID {
			return index
		}
	}

	return -1
}

type menuPromptRow struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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
}

func toMenuPromptRows(menus []*rbacentity.Menu) []menuPromptRow {
	rows := make([]menuPromptRow, 0, len(menus))
	for _, menu := range menus {
		if menu == nil {
			continue
		}

		rows = append(rows, menuPromptRow{
			ID:        menu.ID,
			CreatedAt: menu.CreatedAt,
			UpdatedAt: menu.UpdatedAt,
			Title:     menu.Title,
			TitleEn:   menu.TitleEn,
			TitleJa:   menu.TitleJa,
			Icon:      menu.Icon,
			Path:      menu.Path,
			Name:      menu.Name,
			Component: menu.Component,
			KeepAlive: menu.KeepAlive,
			ParentID:  menu.ParentID,
			SortOrder: menu.SortOrder,
		})
	}

	return rows
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

// RoleMenus 读取角色的直连菜单树与继承菜单树。
func (s *MenuService) RoleMenus(ctx context.Context, roleID uint) (RoleMenusResp, error) {
	role, err := s.roles.FindIDByID(ctx, roleID)
	if err != nil || role == nil {
		return RoleMenusResp{}, rbacerrors.ErrRoleNotFound
	}

	directMenus, err := s.menus.ListByRoleInherited(ctx, role.ID, false)
	if err != nil {
		return RoleMenusResp{}, rbacerrors.ErrGetFailed
	}

	inheritMenus, err := s.menus.ListByRoleInherited(ctx, role.ID, true)
	if err != nil {
		return RoleMenusResp{}, rbacerrors.ErrGetFailed
	}

	return RoleMenusResp{
		DirectMenus:  rbacentity.BuildMenuTree(directMenus),
		InheritMenus: rbacentity.BuildMenuTree(inheritMenus),
	}, nil
}

// UserMenus 读取用户当前角色对应的菜单树；用户未设置当前角色时返回空列表。
func (s *MenuService) UserMenus(ctx context.Context, userID uint) ([]*rbacentity.Menu, error) {
	roleID, found, err := s.userRoles.CurrentRoleID(ctx, userID)
	if err != nil || !found {
		return nil, rbacerrors.ErrUserNotFound
	}

	if roleID == 0 {
		return []*rbacentity.Menu{}, nil
	}

	menus, err := s.menus.ListByRole(ctx, roleID)
	if err != nil {
		return nil, rbacerrors.ErrGetFailed
	}

	return rbacentity.BuildMenuTree(menus), nil
}
