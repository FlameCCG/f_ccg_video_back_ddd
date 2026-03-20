package rbac

import (
	"context"
	"strings"

	rbacerrors "f_ccg_video_back_ddd/internal/domain/rbac/domainerr"
	rbacentity "f_ccg_video_back_ddd/internal/domain/rbac/entity"
	rbacrepo "f_ccg_video_back_ddd/internal/domain/rbac/repository"

	"go.uber.org/zap"
)

// ResourceSyncService 接口资源同步用例（供 CLI -rs 使用）。
type ResourceSyncService struct {
	resources rbacrepo.ApiResourceRepository
	routes    RouteLister
	meta      ResourceMetaLoader
	logger    *zap.Logger
}

// NewResourceSyncService 构造接口资源同步用例。
func NewResourceSyncService(
	resources rbacrepo.ApiResourceRepository,
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
		return ResourceSyncResult{}, rbacerrors.ErrNoRouteFound
	}

	metaMap, err := s.meta.Load()
	if err != nil {
		s.logger.Warn("加载 OpenAPI 资源失败，使用默认说明", zap.Error(err))
	}

	items := rbacentity.BuildApiResources(routes, metaMap)

	existing, err := s.resources.ListAll(ctx)
	if err != nil {
		return ResourceSyncResult{}, err
	}

	result := ResourceSyncResult{Total: len(items)}

	existingMap := make(map[string]*rbacentity.ApiResource, len(existing))
	for _, r := range existing {
		existingMap[rbacentity.ResourceKey(r.Method, r.Path)] = r
	}

	keep := make(map[string]struct{}, len(items))

	for _, item := range items {
		key := rbacentity.ResourceKey(item.Method, item.Path)
		keep[key] = struct{}{}

		old, ok := existingMap[key]
		if !ok {
			if err := s.resources.Create(ctx, item); err != nil {
				return result, err
			}

			result.Created++

			continue
		}

		summary, tags := diffResource(old, item)
		if summary == nil && tags == nil {
			continue
		}

		if err := s.resources.UpdateSummaryAndTags(ctx, old.ID, summary, tags); err != nil {
			return result, err
		}

		result.Updated++
	}

	for _, old := range existing {
		if _, ok := keep[rbacentity.ResourceKey(old.Method, old.Path)]; ok {
			continue
		}

		if err := s.resources.DeleteByID(ctx, old.ID); err != nil {
			return result, err
		}

		result.Deleted++
	}

	return result, nil
}

// diffResource 比较既有资源与目标资源，返回需要更新的 summary 与 tags（nil 表示无需更新）。
//
// summary 比较去首尾空白，tags 按 JSON 语义比较。
func diffResource(old, item *rbacentity.ApiResource) (*string, *[]byte) {
	var (
		summary *string
		tags    *[]byte
	)

	if strings.TrimSpace(item.Summary) != strings.TrimSpace(old.Summary) {
		value := item.Summary
		summary = &value
	}

	if !rbacentity.JSONEqual(old.Tags, item.Tags) {
		value := item.Tags
		tags = &value
	}

	return summary, tags
}
