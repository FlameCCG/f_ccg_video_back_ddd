package notification

import "f_ccg_video_back_ddd/internal/domain/shared/enum"

// TypePolicy 是通知类型与前端分类的对应规则。零值即可使用。
type TypePolicy struct{}

// AllTypes 返回参与角标统计与列表展示的全部通知类型。
func (TypePolicy) AllTypes() []enum.Type {
	return []enum.Type{
		enum.NotificationApplyType,
		enum.NotificationVideoLikeType,
		enum.NotificationLikeCommentType,
		enum.NotificationLikeDynamicType,
		enum.NotificationAtType,
		enum.NotificationSystemType,
	}
}

// TypesByCategory 把前端分类映射为通知类型集合。
func (p TypePolicy) TypesByCategory(category string) []enum.Type {
	switch category {
	case "reply":
		return []enum.Type{enum.NotificationApplyType}
	case "like":
		return []enum.Type{
			enum.NotificationVideoLikeType,
			enum.NotificationLikeCommentType,
			enum.NotificationLikeDynamicType,
		}
	case "at":
		return []enum.Type{enum.NotificationAtType}
	case "system":
		return []enum.Type{enum.NotificationSystemType}
	default:
		return p.AllTypes()
	}
}

// SumByCategory 按分类汇总已按类型分组的数量。未知分类沿用 TypesByCategory 的默认全集。
func (p TypePolicy) SumByCategory(grouped map[enum.Type]int64, category string) int64 {
	var total int64

	for _, item := range p.TypesByCategory(category) {
		total += grouped[item]
	}

	return total
}
