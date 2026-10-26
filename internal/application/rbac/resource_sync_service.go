package rbac

import (
	"context"
	"f_ccg_video_back_ddd/pkg/logctx"
	"strings"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	"go.uber.org/zap"
)

// ResourceSyncService 接口资源同步用例（原 service/rbac_service/resource_sync.go，供 CLI -rs 使用）。
type ResourceSyncService struct {
	resources domain.APIResourceRepository
	routes    RouteLister
	meta      ResourceMetaLoader
	logger    *zap.Logger
}

// NewResourceSyncService 构造接口资源同步用例。
func NewResourceSyncService(
	resources domain.APIResourceRepository,
	routes RouteLister,
	meta ResourceMetaLoader,
	logger *zap.Logger,
) *ResourceSyncService {
	return &ResourceSyncService{resources: resources, routes: routes, meta: meta, logger: logger}
}

// SyncAPIResources 把引擎路由清单全量同步进接口资源表（新增/更新说明与标签/删除失效项）。
func (s *ResourceSyncService) SyncAPIResources(ctx context.Context) (ResourceSyncResult, error) {
	routes := s.routes.Routes()
	if len(routes) == 0 {
		return ResourceSyncResult{}, domain.ErrNoRouteFound
	}

	metaMap, err := s.meta.Load()
	if err != nil {
		logctx.With(ctx, s.logger).Warn("加载 OpenAPI 资源失败，使用默认说明", zap.String("event", "rbac.resource_sync_service.sync_api_resources.degraded"), zap.String("operation", "rbac.resource_sync_service.sync_api_resources"), zap.Error(err))
	}

	items := domain.BuildAPIResources(routes, metaMap)

	existing, err := s.resources.ListAll(ctx)
	if err != nil {
		return ResourceSyncResult{}, err
	}

	result := ResourceSyncResult{Total: len(items)}

	existingMap := make(map[string]*domain.APIResource, len(existing))
	for _, r := range existing {
		existingMap[domain.ResourceKey(r.Method, r.Path)] = r
	}

	return finishResourceSyncServiceSyncAPIResources(ctx, s, items, existing, &result, existingMap)
}

// diffResource 比较既有资源与目标资源，返回需要更新的 summary 与 tags（nil 表示无需更新）。
//
// summary 比较去首尾空白（与原实现一致），tags 按 JSON 语义比较。
func diffResource(old, item *domain.APIResource) (*string, *[]byte) {
	var (
		summary *string
		tags    *[]byte
	)

	if strings.TrimSpace(item.Summary) != strings.TrimSpace(old.Summary) {
		value := item.Summary
		summary = &value
	}

	if !domain.JSONEqual(old.Tags, item.Tags) {
		value := item.Tags
		tags = &value
	}

	return summary, tags
}

func finishResourceSyncServiceSyncAPIResources(ctx context.Context, s *ResourceSyncService, items, existing []*domain.APIResource, result *ResourceSyncResult, existingMap map[string]*domain.APIResource) (ResourceSyncResult, error) {
	keep := make(map[string]struct{}, len(items))

	for _, item := range items {
		key := domain.ResourceKey(item.Method, item.Path)
		keep[key] = struct{}{}

		old, ok := existingMap[key]
		if !ok {
			if err := s.resources.Create(ctx, item); err != nil {
				return (*result), err
			}

			result.Created++

			continue
		}

		summary, tags := diffResource(old, item)
		if summary == nil && tags == nil {
			continue
		}

		if err := s.resources.UpdateSummaryAndTags(ctx, old.ID, summary, tags); err != nil {
			return (*result), err
		}

		result.Updated++
	}

	for _, old := range existing {
		if _, ok := keep[domain.ResourceKey(old.Method, old.Path)]; ok {
			continue
		}

		if err := s.resources.DeleteByID(ctx, old.ID); err != nil {
			return (*result), err
		}

		result.Deleted++
	}

	return (*result), nil
}
