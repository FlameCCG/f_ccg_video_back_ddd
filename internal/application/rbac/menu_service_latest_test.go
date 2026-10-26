package rbac_test

import (
	"context"
	"encoding/json"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/rbac"

	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	"gorm.io/gorm"
)

type fakeMenuGenerator struct {
	title    string
	parentID *uint
	menuJSON []byte
}

func (g *fakeMenuGenerator) GenerateMenuSuggestion(
	_ context.Context,
	title string,
	parentID *uint,
	menuTableJSON []byte,
) (string, error) {
	g.title = title
	if parentID == nil {
		g.parentID = nil
	} else {
		value := *parentID
		g.parentID = &value
	}

	g.menuJSON = append([]byte(nil), menuTableJSON...)

	suggestion := &domain.MenuSuggestion{
		TitleEn:   "Users",
		TitleJa:   "ユーザー",
		Icon:      "<svg></svg>",
		Path:      "users",
		Name:      "Users",
		Component: "views/users/index",
		SortOrder: 1,
	}
	data, err := json.Marshal(suggestion)

	return string(data), err
}

func TestMenuServiceCreateOrdersAndGrantsCreatorRole(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("创建者", "")
	{
		err := s.roleRepo.Create(ctx, role)
		require.NoError(t, err, "seed role: %v", err)
	}

	user := po.User{
		Username:      "menu_creator",
		Email:         "menu_creator@example.com",
		Password:      "x",
		CurrentRoleID: role.ID,
	}
	{
		err := s.db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error
		require.NoError(t, err, "seed creator: %v", err)
	}

	for index, title := range []string{"A", "B", "C"} {
		menu := domain.NewMenu(title, "", "", "", "", "", "", false, nil, uint(index+1))
		{
			err := s.menus.Create(ctx, app.CreateMenuParams{
				Title:     menu.Title,
				SortOrder: menu.SortOrder,
				CreatorID: user.ID,
			})
			require.NoError(t, err, "create seed menu %q: %v", title, err)
		}
	}

	{
		err := s.menus.Create(ctx, app.CreateMenuParams{
			Title:     "New",
			SortOrder: 2,
			CreatorID: user.ID,
		})
		require.NoError(t, err, "create menu: %v", err)
	}

	var menus []po.Menu
	{
		err := s.db.Order("sort_order ASC, id ASC").Find(&menus).Error
		require.NoError(t, err, "load menus: %v", err)
	}

	want := []string{"A", "New", "B", "C"}
	require.Len(t, menus, len(want), "menu count = %d, want %d", len(menus), len(want))

	for index, menu := range menus {
		require.Equal(t, menu.Title, want[index], "menu[%d] = (%q,%d), want (%q,%d)", index, menu.Title, menu.SortOrder, want[index], index+1)
		require.Equal(t, menu.SortOrder, uint(index+1), "menu[%d] = (%q,%d), want (%q,%d)", index, menu.Title, menu.SortOrder, want[index], index+1)
	}

	var roleMenus []po.RoleMenu
	{
		err := s.db.Where("role_id = ?", role.ID).Find(&roleMenus).Error
		require.NoError(t, err, "load creator role menus: %v", err)
	}

	require.Len(t, roleMenus, len(want), "creator role menu count = %d, want %d", len(roleMenus), len(want))

	for _, roleMenu := range roleMenus {
		require.False(t, roleMenu.IsInherited, "creator menu permission must be direct")
	}
}

func TestMenuServiceCreateRejectsUnsafeIcon(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	for _, icon := range []string{
		"<script>alert(1)</script>",
		"<svg onload=\"alert(1)\"></svg>",
		"<a href=\"javascript:alert(1)\">x</a>",
	} {
		err := s.menus.Create(ctx, app.CreateMenuParams{Title: "X", Icon: icon})
		require.ErrorIs(t, err, domain.ErrMenuIconUnsafe, "create with icon %q: err = %v, want ErrMenuIconUnsafe", icon, err)
	}

	var count int64
	{
		err := s.db.Model(&po.Menu{}).Count(&count).Error
		require.NoError(t, err, "count menus: %v", err)
	}

	require.EqualValues(t, 0, count, "menu rows = %d, want 0 (unsafe icons must not persist)", count)
}

func TestMenuServiceUpdateRejectsUnsafeIcon(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	unsafe := "<script>alert(1)</script>"

	err := s.menus.Update(ctx, app.UpdateMenuParams{ID: 1, Icon: &unsafe})
	require.ErrorIs(t, err, domain.ErrMenuIconUnsafe, "update with unsafe icon: err = %v, want ErrMenuIconUnsafe", err)

	tooLong := strings.Repeat("a", 2049)

	err = s.menus.Update(ctx, app.UpdateMenuParams{ID: 1, Icon: &tooLong})
	require.ErrorIs(t, err, domain.ErrMenuIconTooLong, "update with oversized icon: err = %v, want ErrMenuIconTooLong", err)
}

func TestMenuServiceCreateAcceptsSafeIconAndPersists(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("创建者", "")
	{
		err := s.roleRepo.Create(ctx, role)
		require.NoError(t, err, "seed role: %v", err)
	}

	user := po.User{
		Username:      "icon_creator",
		Email:         "icon_creator@example.com",
		Password:      "x",
		CurrentRoleID: role.ID,
	}
	{
		err := s.db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error
		require.NoError(t, err, "seed creator: %v", err)
	}

	icon := "  <svg viewBox=\"0 0 24 24\"><path d=\"M0 0h24v24H0z\"/></svg>  "
	{
		err := s.menus.Create(ctx, app.CreateMenuParams{
			Title:     "SafeIcon",
			Icon:      icon,
			CreatorID: user.ID,
		})
		require.NoError(t, err, "create with safe icon: %v", err)
	}

	var menu po.Menu
	{
		err := s.db.First(&menu).Error
		require.NoError(t, err, "load menu: %v", err)
	}

	require.Equal(t, menu.Icon, strings.TrimSpace(icon), "persisted icon = %q, want trimmed %q", menu.Icon, strings.TrimSpace(icon))
}

func TestMenuServiceCreateRollsBackWithoutCreatorRole(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	{
		err := s.menus.Create(ctx, app.CreateMenuParams{
			Title:     "orphan",
			SortOrder: 1,
			CreatorID: 999,
		})
		require.ErrorIs(t, err, domain.ErrMenuCreatorRoleMissing, "create error = %v, want %v", err, domain.ErrMenuCreatorRoleMissing)
	}

	var count int64
	{
		err := s.db.Model(&po.Menu{}).Where("title = ?", "orphan").Count(&count).Error
		require.NoError(t, err, "count rolled back menu: %v", err)
	}

	require.EqualValues(t, 0, count, "rolled back menu count = %d, want 0", count)
}

func TestMenuServiceMoveAndRejectParentCycle(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("创建者", "")
	{
		err := s.roleRepo.Create(ctx, role)
		require.NoError(t, err, "seed role: %v", err)
	}

	user := po.User{Username: "move_creator", Email: "move@example.com", Password: "x", CurrentRoleID: role.ID}
	{
		err := s.db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error
		require.NoError(t, err, "seed creator: %v", err)
	}

	ids := make([]uint, 0, 3)

	for index, title := range []string{"A", "B", "C"} {
		{
			err := s.menus.Create(ctx, app.CreateMenuParams{
				Title:     title,
				SortOrder: uint(index + 1),
				CreatorID: user.ID,
			})
			require.NoError(t, err, "create menu %q: %v", title, err)
		}

		var row po.Menu
		{
			err := s.db.Where("title = ?", title).First(&row).Error
			require.NoError(t, err, "load menu %q: %v", title, err)
		}

		ids = append(ids, row.ID)
	}

	{
		err := s.menus.Move(ctx, ids[2], ids[0], app.MenuMoveBefore)
		require.NoError(t, err, "move menu: %v", err)
	}

	var ordered []po.Menu
	{
		err := s.db.Order("sort_order ASC, id ASC").Find(&ordered).Error
		require.NoError(t, err, "load order: %v", err)
	}

	require.Equal(t, "C", ordered[0].Title, "unexpected order: %+v", ordered)
	require.Equal(t, "A", ordered[1].Title, "unexpected order: %+v", ordered)
	require.Equal(t, "B", ordered[2].Title, "unexpected order: %+v", ordered)

	parent := ids[0]
	{
		err := s.menus.Update(ctx, app.UpdateMenuParams{
			ID:             ids[1],
			ParentID:       &parent,
			ParentProvided: true,
		})
		require.NoError(t, err, "set child parent: %v", err)
	}

	child := ids[1]
	{
		err := s.menus.Update(ctx, app.UpdateMenuParams{
			ID:             ids[0],
			ParentID:       &child,
			ParentProvided: true,
		})
		require.ErrorIs(t, err, domain.ErrMenuParentCycle, "parent cycle error = %v, want %v", err, domain.ErrMenuParentCycle)
	}
}

func TestMenuServiceGenerateUsesCompleteMenuData(t *testing.T) {
	t.Parallel()

	s := newSuite(t)
	ctx := context.Background()

	menu := domain.NewMenu("系统管理", "System", "システム", "<svg></svg>", "/system", "System", "Layout", false, nil, 1)
	{
		err := s.db.Create(&po.Menu{
			Title:     menu.Title,
			TitleEn:   menu.TitleEn,
			TitleJa:   menu.TitleJa,
			Icon:      menu.Icon,
			Path:      menu.Path,
			Name:      menu.Name,
			Component: menu.Component,
			SortOrder: menu.SortOrder,
		}).Error
		require.NoError(t, err, "seed menu: %v", err)
	}

	suggestion, err := s.menus.Generate(ctx, " 用户管理 ", nil)
	require.NoError(t, err, "generate menu: %v", err)

	require.Equal(t, "Users", suggestion.Name, "unexpected suggestion: %+v", suggestion)

	require.Equal(t, "用户管理", s.generator.title, "unexpected generator input: title=%q parent=%v", s.generator.title, s.generator.parentID)
	require.Nil(t, s.generator.parentID, "unexpected generator input: title=%q parent=%v", s.generator.title, s.generator.parentID)

	require.True(t, json.Valid(s.generator.menuJSON), "menu prompt data is not JSON: %s", s.generator.menuJSON)

	var rows []map[string]any
	{
		err := json.Unmarshal(s.generator.menuJSON, &rows)
		require.NoError(t, err, "decode menu prompt: %v", err)
	}

	require.Len(t, rows, 1, "unexpected menu prompt rows: %+v", rows)
	require.EqualValues(t, "系统管理", rows[0]["title"], "unexpected menu prompt rows: %+v", rows)
	require.NotNil(t, rows[0]["createdAt"], "unexpected menu prompt rows: %+v", rows)
}
