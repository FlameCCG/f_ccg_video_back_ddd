package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	rbacerrors "f_ccg_video_back_ddd/internal/domain/rbac/domainerr"
	rbacentity "f_ccg_video_back_ddd/internal/domain/rbac/entity"
	rbacvo "f_ccg_video_back_ddd/internal/domain/rbac/valueobject"
)

// Generate 根据中文标题、父菜单和完整 menus 数据生成其余可编辑字段。
func (s *MenuService) Generate(
	ctx context.Context,
	title string,
	parentID *uint,
) (*rbacvo.MenuSuggestion, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, rbacerrors.ErrMenuGenerationTitleEmpty
	}

	parentID = normalizedParentID(parentID)
	if parentID != nil {
		parent, err := s.menus.FindByID(ctx, *parentID)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", rbacerrors.ErrReadParentMenuFailed, err)
		}
		if parent == nil {
			return nil, rbacerrors.ErrParentMenuNotFound
		}
	}

	menus, err := s.menus.ListForGeneration(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", rbacerrors.ErrReadMenuDataFailed, err)
	}

	menuTableJSON, err := json.Marshal(toMenuPromptRows(menus))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", rbacerrors.ErrPrepareMenuDataFailed, err)
	}

	if s.generator == nil {
		return nil, rbacerrors.ErrMenuGeneratorUnavailable
	}

	text, err := s.generator.GenerateMenuSuggestion(ctx, title, parentID, menuTableJSON)
	if err != nil {
		return nil, err
	}
	return parseMenuSuggestion(text)
}

type menuPromptRow struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Title     string    `json:"title"`
	TitleEn   string    `json:"titleEn"`
	TitleJa   string    `json:"titleJa"`
	Icon      string    `json:"icon"`
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	Component string    `json:"component"`
	KeepAlive bool      `json:"keepAlive"`
	ParentID  *uint     `json:"parentId"`
	SortOrder uint      `json:"sortOrder"`
}

func toMenuPromptRows(menus []*rbacentity.Menu) []menuPromptRow {
	rows := make([]menuPromptRow, 0, len(menus))
	for _, menu := range menus {
		if menu == nil {
			continue
		}

		rows = append(rows, menuPromptRow{
			ID:        menu.ID,
			CreatedAt: menu.CreatedAt,
			UpdatedAt: menu.UpdatedAt,
			Title:     menu.Title,
			TitleEn:   menu.TitleEn,
			TitleJa:   menu.TitleJa,
			Icon:      menu.Icon,
			Path:      menu.Path,
			Name:      menu.Name,
			Component: menu.Component,
			KeepAlive: menu.KeepAlive,
			ParentID:  menu.ParentID,
			SortOrder: menu.SortOrder,
		})
	}

	return rows
}
