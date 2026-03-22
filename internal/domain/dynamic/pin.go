package dynamic

// PinPlan 描述一次置顶/取消置顶操作的执行计划。
type PinPlan struct {
	// ClearOwnPinned 是否需要先清除该用户已置顶的视频与动态。
	ClearOwnPinned bool
	// Pinned 目标作品最终是否处于置顶状态。
	Pinned bool
}

// PinPolicy 置顶互斥计划与视频置顶资格。零值即可使用。
type PinPolicy struct{}

// BuildPinPlan 按「同一用户同一时间只允许置顶一个作品」的互斥规则生成置顶计划。
//
// 置顶时必须先清空该用户已置顶的视频与动态，再置顶目标作品；
// 取消置顶只影响目标作品本身，无需清理其它作品。
// 规则：对视频与动态双向生效。
func (PinPolicy) BuildPinPlan(pinned bool) PinPlan {
	return PinPlan{ClearOwnPinned: pinned, Pinned: pinned}
}

// EnsureVideoPinnable 校验只有作者本人可以置顶自己的已发布视频。
// username 为空表示用户不存在；owned 为 false 表示目标不是该作者的已发布视频。
func (PinPolicy) EnsureVideoPinnable(username string, owned bool) error {
	if username == "" {
		return ErrUserNotFound
	}

	if !owned {
		return ErrVideoPinForbidden
	}

	return nil
}
