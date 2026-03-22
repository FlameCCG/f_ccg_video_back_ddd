package identity

import (
	"slices"
)

// FallbackDefaultUserBannerID 站点未配置注册默认横幅时的兜底 ID。
const FallbackDefaultUserBannerID uint = 19

// 系统默认用户横幅列表的操作类型。
const (
	// UserBannerActionAdd 追加到系统默认列表。
	UserBannerActionAdd = 1
	// UserBannerActionDelete 从系统默认列表移除。
	UserBannerActionDelete = 2
)

// BannerPolicy 维护系统默认横幅的选择与列表编辑规则。
type BannerPolicy struct{}

type DefaultBannerChange struct {
	PreviousID uint
	RegisterID uint
	Defaults   []uint
}

// ReplaceRegisterDefault 保留默认列表原顺序及历史值，只在需要时追加旧默认。
func (p BannerPolicy) ReplaceRegisterDefault(configured uint, defaults []uint, nextID uint) (DefaultBannerChange, error) {
	previous := p.ResolveDefaultID(configured)
	if nextID == previous {
		return DefaultBannerChange{}, ErrBannerAlreadyRegisterDefault
	}
	next := slices.Clone(defaults)
	if previous > 0 && !slices.Contains(next, previous) {
		next = append(next, previous)
	}
	return DefaultBannerChange{PreviousID: previous, RegisterID: nextID, Defaults: next}, nil
}

// EditDefaults 一次决定默认列表增删、重复项及注册默认保护。
func (p BannerPolicy) EditDefaults(configured []uint, registerDefault uint, requested []uint, action int) ([]uint, error) {
	if err := p.EnsureRegisterDefaultUntouched(requested, registerDefault); err != nil {
		return nil, err
	}
	current := p.NormalizeIDs(configured)
	switch action {
	case UserBannerActionAdd:
		for _, id := range p.NormalizeIDs(requested) {
			if slices.Contains(current, id) {
				return nil, ErrBannerAlreadyInDefaults
			}
		}
		return p.Merge(current, requested), nil
	case UserBannerActionDelete:
		return p.Remove(current, requested), nil
	default:
		return nil, ErrBannerActionInvalid
	}
}

func (BannerPolicy) RequireID(id uint) error {
	if id == 0 {
		return ErrBannerIDRequired
	}
	return nil
}

func (BannerPolicy) RequireIDs(ids []uint) error {
	if len(ids) == 0 {
		return ErrBannerIDsRequired
	}
	return nil
}

func (BannerPolicy) EnsureAvailable(expected, actual int) error {
	if expected != actual {
		return ErrUserBannerNotFound
	}
	return nil
}

func (BannerPolicy) EnsureFound(found bool) error {
	if !found {
		return ErrUserBannerNotFound
	}
	return nil
}

func (p BannerPolicy) EnsureSystemDefaultID(id uint, configured []uint) error {
	if err := p.RequireID(id); err != nil {
		return err
	}
	if !slices.Contains(configured, id) {
		return ErrSystemDefaultBannerNotFound
	}
	return nil
}

func (BannerPolicy) EnsureSystemDefaultVisible(count int) error {
	if count != 1 {
		return ErrSystemDefaultBannerNotFound
	}
	return nil
}

// NormalizeIDs 去零、去重并保持原顺序；空集合返回空切片而非 nil。
func (BannerPolicy) NormalizeIDs(ids []uint) []uint {
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

// Merge 把 appendIDs 追加到 base 尾部（去零去重，保持先后顺序）。
func (p BannerPolicy) Merge(base, appendIDs []uint) []uint {
	merged := p.NormalizeIDs(base)

	seen := make(map[uint]struct{}, len(merged))
	for _, id := range merged {
		seen[id] = struct{}{}
	}

	for _, id := range p.NormalizeIDs(appendIDs) {
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		merged = append(merged, id)
	}

	return merged
}

// Remove 从 base 中移除 removeIDs。
func (p BannerPolicy) Remove(base, removeIDs []uint) []uint {
	base = p.NormalizeIDs(base)
	if len(base) == 0 {
		return []uint{}
	}

	removeSet := make(map[uint]struct{}, len(removeIDs))
	for _, id := range p.NormalizeIDs(removeIDs) {
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

// ResolveDefaultID 依据站点配置解析注册默认横幅 ID，缺失时返回兜底值。
func (BannerPolicy) ResolveDefaultID(configured uint) uint {
	if configured > 0 {
		return configured
	}

	return FallbackDefaultUserBannerID
}

// SystemDefaults 汇总系统默认横幅集合：配置列表 + 注册默认横幅（去重、注册默认排最后）。
//
// 语义。
func (p BannerPolicy) SystemDefaults(configuredIDs []uint, registerDefaultID uint) []uint {
	bannerIDs := p.NormalizeIDs(configuredIDs)
	if registerDefaultID == 0 {
		return bannerIDs
	}

	result := append([]uint(nil), bannerIDs...)
	if slices.Contains(result, registerDefaultID) {
		return result
	}

	return append(result, registerDefaultID)
}

// EnsureRegisterDefaultUntouched 校验待操作的横幅列表不包含注册默认横幅。
func (p BannerPolicy) EnsureRegisterDefaultUntouched(bannerIDs []uint, registerDefaultID uint) error {
	if registerDefaultID == 0 {
		return nil
	}

	if slices.Contains(p.NormalizeIDs(bannerIDs), registerDefaultID) {
		return ErrRegisterDefaultBannerLocked
	}

	return nil
}
