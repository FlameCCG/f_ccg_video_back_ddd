package rbac

import (
	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
	"time"
)

// MenuDTO preserves the existing response contract.
type MenuDTO struct {
	ID        uint       `json:"id"`
	CreatedAt time.Time  `json:"-"`
	UpdatedAt time.Time  `json:"-"`
	Title     string     `json:"title"`
	TitleEn   string     `json:"titleEn"`
	TitleJa   string     `json:"titleJa"`
	Icon      string     `json:"icon"`
	Path      string     `json:"path"`
	Name      string     `json:"name"`
	Component string     `json:"component"`
	KeepAlive bool       `json:"keepAlive"`
	ParentID  *uint      `json:"parentId"`
	SortOrder uint       `json:"sortOrder"`
	Children  []*MenuDTO `json:"children,omitempty"`
}

func ToMenuDTO(value *rbacdomain.Menu) *MenuDTO {
	if value == nil {
		return nil
	}
	return &MenuDTO{
		ID:        value.ID(),
		Title:     value.Title(),
		TitleEn:   value.TitleEn(),
		TitleJa:   value.TitleJa(),
		Icon:      value.Icon(),
		Path:      value.Path(),
		Name:      value.Name(),
		Component: value.Component(),
		KeepAlive: value.KeepAlive(),
		ParentID:  value.ParentID(),
		SortOrder: value.SortOrder(),
		Children:  ToMenuDTOs(value.Children()),
	}
}
func ToMenuDTOs(values []*rbacdomain.Menu) []*MenuDTO {
	if values == nil {
		return nil
	}
	result := make([]*MenuDTO, len(values))
	for i, v := range values {
		result[i] = ToMenuDTO(v)
	}
	return result
}

// RoleDTO preserves the existing response contract.
type RoleDTO struct {
	ID        uint       `json:"id"`
	CreatedAt time.Time  `json:"-"`
	UpdatedAt time.Time  `json:"-"`
	Name      string     `json:"name"`
	Desc      string     `json:"desc"`
	Menus     []*MenuDTO `json:"menus,omitempty"`
}

func ToRoleDTO(value *rbacdomain.Role) *RoleDTO {
	if value == nil {
		return nil
	}
	return &RoleDTO{
		ID:    value.ID(),
		Name:  value.Name(),
		Desc:  value.Desc(),
		Menus: ToMenuDTOs(value.Menus()),
	}
}
func ToRoleDTOs(values []*rbacdomain.Role) []*RoleDTO {
	if values == nil {
		return nil
	}
	result := make([]*RoleDTO, len(values))
	for i, v := range values {
		result[i] = ToRoleDTO(v)
	}
	return result
}
