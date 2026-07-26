package po

import (
	"context"
	"slices"
)

// FallbackDefaultUserBannerID 配置缺失时的兜底默认用户横幅 ID（与原实现一致）。
const FallbackDefaultUserBannerID uint = 19

// ctxKeyDefaultUserBannerID 在 context 中传递默认用户横幅 ID 的键。
//
// 原实现由 models/mysqlEntity 直接读取 global.Config()，使持久化对象反向依赖全局配置；
// 新架构由仓储层从注入的配置解析后写入 ctx，PO 的 GORM 钩子从 ctx 读取。
type ctxKeyDefaultUserBannerID struct{}

// WithDefaultUserBannerID 把默认用户横幅 ID 注入 context，供 User.AfterCreate 钩子读取。
func WithDefaultUserBannerID(ctx context.Context, bannerID uint) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	if bannerID == 0 {
		return ctx
	}

	return context.WithValue(ctx, ctxKeyDefaultUserBannerID{}, bannerID)
}

// DefaultUserBannerIDFromContext 读取 context 中的默认用户横幅 ID，缺失时返回兜底值。
func DefaultUserBannerIDFromContext(ctx context.Context) uint {
	if ctx != nil {
		if bannerID, ok := ctx.Value(ctxKeyDefaultUserBannerID{}).(uint); ok && bannerID > 0 {
			return bannerID
		}
	}

	return FallbackDefaultUserBannerID
}

// NormalizeBannerIDs 去零去重，保持原顺序（与原 models/mysqlEntity.normalizeBannerIDs 一致）。
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

// ResolveDefaultUserBannerID 依据站点配置解析注册默认横幅 ID，缺失时返回兜底值。
func ResolveDefaultUserBannerID(configured uint) uint {
	if configured > 0 {
		return configured
	}

	return FallbackDefaultUserBannerID
}

// SystemDefaultUserBannerIDs 汇总站点默认横幅集合：配置列表 + 注册默认横幅（去重）。
// 与原 models/mysqlEntity.SystemDefaultUserBannerIDs 语义一致。
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
