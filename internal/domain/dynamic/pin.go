package dynamic

// PinPlan 描述一次置顶/取消置顶操作的执行计划。
type PinPlan struct {
	// ClearOwnPinned 是否需要先清除该用户已置顶的视频与动态。
	ClearOwnPinned bool
	// Pinned 目标作品最终是否处于置顶状态。
	Pinned bool
}

// BuildPinPlan 按「同一用户同一时间只允许置顶一个作品」的互斥规则生成置顶计划。
//
// 置顶时必须先清空该用户已置顶的视频与动态，再置顶目标作品；
// 取消置顶只影响目标作品本身，无需清理其它作品。
// 规则对视频与动态双向生效（原 DynamicService.Pin 与 PinVideo 各自实现了同一条规则）。
func BuildPinPlan(pinned bool) PinPlan {
	return PinPlan{ClearOwnPinned: pinned, Pinned: pinned}
}
