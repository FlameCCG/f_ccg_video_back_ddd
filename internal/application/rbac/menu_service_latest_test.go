package rbac_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/rbac"
	domain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

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
	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("创建者", "")
	if err := s.roleRepo.Create(ctx, role); err != nil {
		t.Fatalf("seed role: %v", err)
	}

	user := po.User{
		Username:      "menu_creator",
		Email:         "menu_creator@example.com",
		Password:      "x",
		CurrentRoleID: role.ID,
	}
	if err := s.db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("seed creator: %v", err)
	}

	for index, title := range []string{"A", "B", "C"} {
		menu := domain.NewMenu(title, "", "", "", "", "", "", false, nil, uint(index+1))
		if err := s.menus.Create(ctx, app.CreateMenuParams{
			Title:     menu.Title,
			SortOrder: menu.SortOrder,
			CreatorID: user.ID,
		}); err != nil {
			t.Fatalf("create seed menu %q: %v", title, err)
		}
	}

	if err := s.menus.Create(ctx, app.CreateMenuParams{
		Title:     "New",
		SortOrder: 2,
		CreatorID: user.ID,
	}); err != nil {
		t.Fatalf("create menu: %v", err)
	}

	var menus []po.Menu
	if err := s.db.Order("sort_order ASC, id ASC").Find(&menus).Error; err != nil {
		t.Fatalf("load menus: %v", err)
	}

	want := []string{"A", "New", "B", "C"}
	if len(menus) != len(want) {
		t.Fatalf("menu count = %d, want %d", len(menus), len(want))
	}
	for index, menu := range menus {
		if menu.Title != want[index] || menu.SortOrder != uint(index+1) {
			t.Fatalf("menu[%d] = (%q,%d), want (%q,%d)", index, menu.Title, menu.SortOrder, want[index], index+1)
		}
	}

	var roleMenus []po.RoleMenu
	if err := s.db.Where("role_id = ?", role.ID).Find(&roleMenus).Error; err != nil {
		t.Fatalf("load creator role menus: %v", err)
	}
	if len(roleMenus) != len(want) {
		t.Fatalf("creator role menu count = %d, want %d", len(roleMenus), len(want))
	}
	for _, roleMenu := range roleMenus {
		if roleMenu.IsInherited {
			t.Fatal("creator menu permission must be direct")
		}
	}
}

func TestMenuServiceCreateRejectsUnsafeIcon(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	for _, icon := range []string{
		"<script>alert(1)</script>",
		"<svg onload=\"alert(1)\"></svg>",
		"<a href=\"javascript:alert(1)\">x</a>",
	} {
		err := s.menus.Create(ctx, app.CreateMenuParams{Title: "X", Icon: icon})
		if !errors.Is(err, domain.ErrMenuIconUnsafe) {
			t.Fatalf("create with icon %q: err = %v, want ErrMenuIconUnsafe", icon, err)
		}
	}

	var count int64
	if err := s.db.Model(&po.Menu{}).Count(&count).Error; err != nil {
		t.Fatalf("count menus: %v", err)
	}
	if count != 0 {
		t.Fatalf("menu rows = %d, want 0 (unsafe icons must not persist)", count)
	}
}

func TestMenuServiceUpdateRejectsUnsafeIcon(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	unsafe := "<script>alert(1)</script>"
	err := s.menus.Update(ctx, app.UpdateMenuParams{ID: 1, Icon: &unsafe})
	if !errors.Is(err, domain.ErrMenuIconUnsafe) {
		t.Fatalf("update with unsafe icon: err = %v, want ErrMenuIconUnsafe", err)
	}

	tooLong := strings.Repeat("a", 2049)
	err = s.menus.Update(ctx, app.UpdateMenuParams{ID: 1, Icon: &tooLong})
	if !errors.Is(err, domain.ErrMenuIconTooLong) {
		t.Fatalf("update with oversized icon: err = %v, want ErrMenuIconTooLong", err)
	}
}

func TestMenuServiceCreateAcceptsSafeIconAndPersists(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("创建者", "")
	if err := s.roleRepo.Create(ctx, role); err != nil {
		t.Fatalf("seed role: %v", err)
	}

	user := po.User{
		Username:      "icon_creator",
		Email:         "icon_creator@example.com",
		Password:      "x",
		CurrentRoleID: role.ID,
	}
	if err := s.db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("seed creator: %v", err)
	}

	icon := "  <svg viewBox=\"0 0 24 24\"><path d=\"M0 0h24v24H0z\"/></svg>  "
	if err := s.menus.Create(ctx, app.CreateMenuParams{
		Title:     "SafeIcon",
		Icon:      icon,
		CreatorID: user.ID,
	}); err != nil {
		t.Fatalf("create with safe icon: %v", err)
	}

	var menu po.Menu
	if err := s.db.First(&menu).Error; err != nil {
		t.Fatalf("load menu: %v", err)
	}
	if menu.Icon != strings.TrimSpace(icon) {
		t.Fatalf("persisted icon = %q, want trimmed %q", menu.Icon, strings.TrimSpace(icon))
	}
}

func TestMenuServiceCreateRollsBackWithoutCreatorRole(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	if err := s.menus.Create(ctx, app.CreateMenuParams{
		Title:     "orphan",
		SortOrder: 1,
		CreatorID: 999,
	}); err != domain.ErrMenuCreatorRoleMissing {
		t.Fatalf("create error = %v, want %v", err, domain.ErrMenuCreatorRoleMissing)
	}

	var count int64
	if err := s.db.Model(&po.Menu{}).Where("title = ?", "orphan").Count(&count).Error; err != nil {
		t.Fatalf("count rolled back menu: %v", err)
	}
	if count != 0 {
		t.Fatalf("rolled back menu count = %d, want 0", count)
	}
}

func TestMenuServiceMoveAndRejectParentCycle(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	role := domain.NewRole("创建者", "")
	if err := s.roleRepo.Create(ctx, role); err != nil {
		t.Fatalf("seed role: %v", err)
	}
	user := po.User{Username: "move_creator", Email: "move@example.com", Password: "x", CurrentRoleID: role.ID}
	if err := s.db.Session(&gorm.Session{SkipHooks: true}).Create(&user).Error; err != nil {
		t.Fatalf("seed creator: %v", err)
	}

	ids := make([]uint, 0, 3)
	for index, title := range []string{"A", "B", "C"} {
		if err := s.menus.Create(ctx, app.CreateMenuParams{
			Title:     title,
			SortOrder: uint(index + 1),
			CreatorID: user.ID,
		}); err != nil {
			t.Fatalf("create menu %q: %v", title, err)
		}

		var row po.Menu
		if err := s.db.Where("title = ?", title).First(&row).Error; err != nil {
			t.Fatalf("load menu %q: %v", title, err)
		}
		ids = append(ids, row.ID)
	}

	if err := s.menus.Move(ctx, ids[2], ids[0], app.MenuMoveBefore); err != nil {
		t.Fatalf("move menu: %v", err)
	}

	var ordered []po.Menu
	if err := s.db.Order("sort_order ASC, id ASC").Find(&ordered).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if ordered[0].Title != "C" || ordered[1].Title != "A" || ordered[2].Title != "B" {
		t.Fatalf("unexpected order: %+v", ordered)
	}

	parent := ids[0]
	if err := s.menus.Update(ctx, app.UpdateMenuParams{
		ID:             ids[1],
		ParentID:       &parent,
		ParentProvided: true,
	}); err != nil {
		t.Fatalf("set child parent: %v", err)
	}

	child := ids[1]
	if err := s.menus.Update(ctx, app.UpdateMenuParams{
		ID:             ids[0],
		ParentID:       &child,
		ParentProvided: true,
	}); err != domain.ErrMenuParentCycle {
		t.Fatalf("parent cycle error = %v, want %v", err, domain.ErrMenuParentCycle)
	}
}

func TestMenuServiceGenerateUsesCompleteMenuData(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	menu := domain.NewMenu("系统管理", "System", "システム", "<svg></svg>", "/system", "System", "Layout", false, nil, 1)
	if err := s.db.Create(&po.Menu{
		Title:     menu.Title,
		TitleEn:   menu.TitleEn,
		TitleJa:   menu.TitleJa,
		Icon:      menu.Icon,
		Path:      menu.Path,
		Name:      menu.Name,
		Component: menu.Component,
		SortOrder: menu.SortOrder,
	}).Error; err != nil {
		t.Fatalf("seed menu: %v", err)
	}

	suggestion, err := s.menus.Generate(ctx, " 用户管理 ", nil)
	if err != nil {
		t.Fatalf("generate menu: %v", err)
	}
	if suggestion.Name != "Users" {
		t.Fatalf("unexpected suggestion: %+v", suggestion)
	}
	if s.generator.title != "用户管理" || s.generator.parentID != nil {
		t.Fatalf("unexpected generator input: title=%q parent=%v", s.generator.title, s.generator.parentID)
	}
	if !json.Valid(s.generator.menuJSON) {
		t.Fatalf("menu prompt data is not JSON: %s", s.generator.menuJSON)
	}

	var rows []map[string]any
	if err := json.Unmarshal(s.generator.menuJSON, &rows); err != nil {
		t.Fatalf("decode menu prompt: %v", err)
	}
	if len(rows) != 1 || rows[0]["title"] != "系统管理" || rows[0]["createdAt"] == nil {
		t.Fatalf("unexpected menu prompt rows: %+v", rows)
	}
}
