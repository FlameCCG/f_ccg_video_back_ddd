package rbac

const (
	MenuMoveBefore = "before"
	MenuMoveAfter  = "after"
)

// MenuOrder 规划同层插入、拖拽和父子挂接。它没有依赖，调用方用零值即可。
type MenuOrder struct{}

// MenuInsertionPlan 给出插入后的同级顺序及新菜单位置；零 ID 留给应用在创建后回填。
type MenuInsertionPlan struct {
	Sequence  []uint
	SortOrder uint
}

// PlanInsertion 计算新菜单插入后的同层顺序。不修改入参。
func (MenuOrder) PlanInsertion(menuID, requested uint, siblings []uint) MenuInsertionPlan {
	target := (MenuOrder{}).normalizedTargetSort(requested, len(siblings))

	return MenuInsertionPlan{Sequence: (MenuOrder{}).insertID(siblings, menuID, int(target-1)), SortOrder: target}
}

// ValidPosition 只接受 before 与 after。
func (MenuOrder) ValidPosition(position string) error {
	if position != MenuMoveBefore && position != MenuMoveAfter {
		return ErrMenuMovePosition
	}

	return nil
}

// EnsureMoveAllowed 在读取同层顺序之前检查拖拽资格，保留业务错误的优先级。
func (order MenuOrder) EnsureMoveAllowed(moving, target *Menu, position string) error {
	if moving == nil || target == nil {
		return ErrMenuNotFound
	}
	if err := order.ValidPosition(position); err != nil {
		return err
	}
	if !order.SameParent(moving.ParentID(), target.ParentID()) {
		return ErrMenuMoveAcrossLevels
	}
	return nil
}

// PlanMove 只允许同级拖拽，不修改传入菜单或顺序。
func (MenuOrder) PlanMove(moving, target *Menu, siblings []uint, position string) ([]uint, error) {
	if err := (MenuOrder{}).EnsureMoveAllowed(moving, target, position); err != nil {
		return nil, err
	}

	sequence := (MenuOrder{}).removeID(siblings, moving.ID())
	index := (MenuOrder{}).index(sequence, target.ID())
	if index < 0 {
		return nil, ErrMenuNotFound
	}
	if position == MenuMoveAfter {
		index++
	}

	return (MenuOrder{}).insertID(sequence, moving.ID(), index), nil
}

// MenuPlacementPlan 同时描述原层级压紧和目标层级插入，保存顺序由应用控制。
type MenuPlacementPlan struct {
	SourceOrder, DestinationOrder []uint
	ParentChanged                 bool
}

// PlanPlacement 计算换父或改排序后的两层顺序。不修改菜单。
func (MenuOrder) PlanPlacement(menu *Menu, nextParent *uint, sortOrder *uint, source, destination []uint) MenuPlacementPlan {
	changed := !(MenuOrder{}).SameParent(menu.ParentID(), nextParent)
	oldPosition := (MenuOrder{}).index(source, menu.ID()) + 1
	source = (MenuOrder{}).removeID(source, menu.ID())
	if !changed {
		destination = source
	}

	requested := uint(0)
	if sortOrder != nil {
		requested = *sortOrder
	} else if !changed && oldPosition > 0 {
		requested = uint(oldPosition)
	}

	inserted := (MenuOrder{}).PlanInsertion(menu.ID(), requested, destination)
	plan := MenuPlacementPlan{DestinationOrder: inserted.Sequence, ParentChanged: changed}
	if changed {
		plan.SourceOrder = source
	}

	return plan
}

// NormalizeParent 把空指针和 0 视为根，并复制非根父级，避免调用方改到内部状态。
func (MenuOrder) NormalizeParent(parentID *uint) *uint {
	if parentID == nil || *parentID == 0 {
		return nil
	}

	value := *parentID

	return &value
}

// SameParent 判断两个父级是否同一层。nil 与 0 都表示根。
func (MenuOrder) SameParent(left, right *uint) bool {
	leftRoot := left == nil || *left == 0
	rightRoot := right == nil || *right == 0
	if leftRoot || rightRoot {
		return leftRoot && rightRoot
	}

	return *left == *right
}

// Tree 构建菜单树；入参为空时返回 nil，接口响应为 data:null。
func (MenuOrder) Tree(menus []*Menu) []*Menu {
	menuMap := make(map[uint]*Menu)
	var roots []*Menu
	for i := range menus {
		menuMap[menus[i].id] = menus[i]
	}

	for i := range menus {
		menu := menus[i]
		if menu.parentID == nil || *menu.parentID == 0 {
			roots = append(roots, menu)
			continue
		}

		if parent, ok := menuMap[*menu.parentID]; ok {
			parent.children = append(parent.children, menu)
			continue
		}
		// 兜底：父菜单不存在时也视为根节点，避免丢失
		roots = append(roots, menu)
	}

	return roots
}

func (MenuOrder) normalizedTargetSort(requested uint, siblingCount int) uint {
	maximum := uint(siblingCount + 1)
	if requested == 0 || requested > maximum {
		return maximum
	}

	return requested
}

func (MenuOrder) removeID(menuIDs []uint, menuID uint) []uint {
	result := make([]uint, 0, len(menuIDs))
	for _, id := range menuIDs {
		if id != menuID {
			result = append(result, id)
		}
	}

	return result
}

func (MenuOrder) insertID(menuIDs []uint, menuID uint, index int) []uint {
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

func (MenuOrder) index(menuIDs []uint, menuID uint) int {
	for index, id := range menuIDs {
		if id == menuID {
			return index
		}
	}

	return -1
}
