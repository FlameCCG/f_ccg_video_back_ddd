package service

import (
	rbacerrors "f_ccg_video_back_ddd/internal/domain/rbac/domainerr"
	rbacentity "f_ccg_video_back_ddd/internal/domain/rbac/entity"
)

const (
	MenuMoveBefore = "before"
	MenuMoveAfter  = "after"
)

// MenuInsertionPlan 给出插入后的同级顺序及新菜单位置；零 ID 留给应用在创建后回填。
type MenuInsertionPlan struct {
	Sequence  []uint
	SortOrder uint
}

func PlanMenuInsertion(menuID, requested uint, siblings []uint) MenuInsertionPlan {
	target := normalizedTargetSort(requested, len(siblings))
	return MenuInsertionPlan{Sequence: insertMenuID(siblings, menuID, int(target-1)), SortOrder: target}
}

// PlanMenuMove 只允许同级拖拽，不修改传入快照或顺序。
func PlanMenuMove(moving, target *rbacentity.Menu, siblings []uint, position string) ([]uint, error) {
	if moving == nil || target == nil {
		return nil, rbacerrors.ErrMenuNotFound
	}
	if position != MenuMoveBefore && position != MenuMoveAfter {
		return nil, rbacerrors.ErrMenuMovePosition
	}
	if !SameMenuParent(moving.ParentID, target.ParentID) {
		return nil, rbacerrors.ErrMenuMoveAcrossLevels
	}
	sequence := removeMenuID(siblings, moving.ID)
	index := menuIDIndex(sequence, target.ID)
	if index < 0 {
		return nil, rbacerrors.ErrMenuNotFound
	}
	if position == MenuMoveAfter {
		index++
	}
	return insertMenuID(sequence, moving.ID, index), nil
}

// MenuPlacementPlan 同时描述原层级压紧和目标层级插入，保存顺序由应用控制。
type MenuPlacementPlan struct {
	SourceOrder, DestinationOrder []uint
	ParentChanged                 bool
}

func PlanMenuPlacement(menu *rbacentity.Menu, nextParent *uint, sortOrder *uint, source, destination []uint) MenuPlacementPlan {
	changed := !SameMenuParent(menu.ParentID, nextParent)
	oldPosition := menuIDIndex(source, menu.ID) + 1
	source = removeMenuID(source, menu.ID)
	if !changed {
		destination = source
	}
	requested := uint(0)
	if sortOrder != nil {
		requested = *sortOrder
	} else if !changed && oldPosition > 0 {
		requested = uint(oldPosition)
	}
	inserted := PlanMenuInsertion(menu.ID, requested, destination)
	plan := MenuPlacementPlan{DestinationOrder: inserted.Sequence, ParentChanged: changed}
	if changed {
		plan.SourceOrder = source
	}
	return plan
}
func normalizedTargetSort(requested uint, siblingCount int) uint {
	maximum := uint(siblingCount + 1)
	if requested == 0 || requested > maximum {
		return maximum
	}

	return requested
}

func SameMenuParent(left, right *uint) bool {
	leftRoot := left == nil || *left == 0
	rightRoot := right == nil || *right == 0
	if leftRoot || rightRoot {
		return leftRoot && rightRoot
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
