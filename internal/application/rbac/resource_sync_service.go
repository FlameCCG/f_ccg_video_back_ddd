package rbac

import (
	"context"

	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"

	"go.uber.org/zap"
)

// ResourceSyncService 接口资源同步用例（供 CLI -rs 使用）。
type ResourceSyncService struct {
	resources rbacdomain.ApiResourceRepository
	routes    RouteLister
	meta      ResourceMetaLoader
	logger    *zap.Logger
}

// NewResourceSyncService 构造接口资源同步用例。
func NewResourceSyncService(
	resources rbacdomain.ApiResourceRepository,
	routes RouteLister,
	meta ResourceMetaLoader,
	logger *zap.Logger,
) *ResourceSyncService {
	return &ResourceSyncService{resources: resources, routes: routes, meta: meta, logger: logger}
}

// SyncApiResources 把引擎路由清单全量同步进接口资源表（新增/更新说明与标签/删除失效项）。
func (s *ResourceSyncService) SyncApiResources(ctx context.Context) (ResourceSyncResult, error) {
	routes := s.routes.Routes()
	if len(routes) == 0 {
		return ResourceSyncResult{}, rbacdomain.ErrNoRouteFound
	}

	metaMap, err := s.meta.Load()
	if err != nil {
		s.logger.Warn("加载 OpenAPI 资源失败，使用默认说明", zap.Error(err))
	}

	items := (rbacdomain.ApiResource{}).Build(routes, metaMap)

	existing, err := s.resources.ListAll(ctx)
	if err != nil {
		return ResourceSyncResult{}, err
	}

	result := ResourceSyncResult{Total: len(items)}

	existingMap := make(map[string]*rbacdomain.ApiResource, len(existing))
	for _, r := range existing {
		existingMap[(rbacdomain.ApiResource{}).Key(r.Method, r.Path)] = r
	}

	keep := make(map[string]struct{}, len(items))

	for _, item := range items {
		key := (rbacdomain.ApiResource{}).Key(item.Method, item.Path)
		keep[key] = struct{}{}

		old, ok := existingMap[key]
		if !ok {
			if _, err := s.resources.Create(ctx, item); err != nil {
				return result, err
			}

			result.Created++

			continue
		}

		updated, err := s.resources.UpdateMetadata(ctx, old.ID, old.PlanMetadata(item))
		if err != nil {
			return result, err
		}
		if updated {
			result.Updated++
		}
	}

	for _, old := range existing {
		if _, ok := keep[(rbacdomain.ApiResource{}).Key(old.Method, old.Path)]; ok {
			continue
		}

		if err := s.resources.DeleteByID(ctx, old.ID); err != nil {
			return result, err
		}

		result.Deleted++
	}

	return result, nil
}
