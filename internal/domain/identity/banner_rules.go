package identity

import "slices"

// FallbackDefaultUserBannerID 站点未配置注册默认横幅时的兜底 ID（与原实现一致）。
const FallbackDefaultUserBannerID uint = 19

// 系统默认用户横幅列表的操作类型（与原 api/user_api/user_default_banner.go 一致）。
const (
	// UserBannerActionAdd 追加到系统默认列表。
	UserBannerActionAdd = 1
	// UserBannerActionDelete 从系统默认列表移除。
	UserBannerActionDelete = 2
)

// NormalizeBannerIDs 去零、去重并保持原顺序；空集合返回空切片而非 nil。
func NormalizeBannerIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return []uint{}
	}

	result := make([]uint, 0, len(ids))

	seen := make(map[uint]struct{}, len(ids))

	for _, id := range ids {
		if id == 0 {
			continue
		}

		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		result = append(result, id)
	}

	return result
}

// MergeBannerIDs 把 appendIDs 追加到 base 尾部（去零去重，保持先后顺序）。
func MergeBannerIDs(base, appendIDs []uint) []uint {
	merged := NormalizeBannerIDs(base)

	seen := make(map[uint]struct{}, len(merged))
	for _, id := range merged {
		seen[id] = struct{}{}
	}

	for _, id := range NormalizeBannerIDs(appendIDs) {
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		merged = append(merged, id)
	}

	return merged
}

// RemoveBannerIDs 从 base 中移除 removeIDs。
func RemoveBannerIDs(base, removeIDs []uint) []uint {
	base = NormalizeBannerIDs(base)
	if len(base) == 0 {
		return []uint{}
	}

	removeSet := make(map[uint]struct{}, len(removeIDs))
	for _, id := range NormalizeBannerIDs(removeIDs) {
		removeSet[id] = struct{}{}
	}

	result := make([]uint, 0, len(base))

	for _, id := range base {
		if _, ok := removeSet[id]; ok {
			continue
		}

		result = append(result, id)
	}

	return result
}

// ResolveDefaultUserBannerID 依据站点配置解析注册默认横幅 ID，缺失时返回兜底值。
func ResolveDefaultUserBannerID(configured uint) uint {
	if configured > 0 {
		return configured
	}

	return FallbackDefaultUserBannerID
}

// SystemDefaultUserBannerIDs 汇总系统默认横幅集合：配置列表 + 注册默认横幅（去重、注册默认排最后）。
//
// 语义与原 models/mysqlEntity.SystemDefaultUserBannerIDs 一致。
func SystemDefaultUserBannerIDs(configuredIDs []uint, registerDefaultID uint) []uint {
	bannerIDs := NormalizeBannerIDs(configuredIDs)
	if registerDefaultID == 0 {
		return bannerIDs
	}

	result := append([]uint(nil), bannerIDs...)
	if slices.Contains(result, registerDefaultID) {
		return result
	}

	return append(result, registerDefaultID)
}

// EnsureRegisterDefaultBannerNotTouched 校验待操作的横幅列表不包含注册默认横幅。
func EnsureRegisterDefaultBannerNotTouched(bannerIDs []uint, registerDefaultID uint) error {
	if registerDefaultID == 0 {
		return nil
	}

	if slices.Contains(NormalizeBannerIDs(bannerIDs), registerDefaultID) {
		return ErrRegisterDefaultBannerLocked
	}

	return nil
}
