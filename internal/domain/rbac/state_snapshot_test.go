package rbac

import "testing"

func TestMenuAndRoleSnapshotsCopyNestedState(t *testing.T) {
	parent := uint(1)
	source := MenuSnapshot{ID: 2, ParentID: &parent, Children: []MenuSnapshot{{ID: 3, Title: "child"}}}
	menu := RestoreMenu(source)
	parent = 100
	source.Children[0].Title = "input"
	*menu.ParentID() = 200
	child := menu.Children()[0]
	title := "read"
	if _, err := child.Revise(MenuFieldUpdate{Title: &title}); err != nil {
		t.Fatal(err)
	}
	snapshot := menu.Snapshot()
	snapshot.Children[0].Title = "snapshot"
	if *menu.ParentID() != 1 || menu.Children()[0].Title() != "child" {
		t.Fatal("menu tree escaped encapsulation")
	}
	role := RestoreRole(RoleSnapshot{ID: 4, Menus: []MenuSnapshot{menu.Snapshot()}})
	roleSnapshot := role.Snapshot()
	roleSnapshot.Menus[0].Title = "snapshot"
	menus := role.Menus()
	if _, err := menus[0].Revise(MenuFieldUpdate{Title: &title}); err != nil {
		t.Fatal(err)
	}
	if role.Menus()[0].Title() == "read" || role.Menus()[0].Title() == "snapshot" {
		t.Fatal("role menus escaped encapsulation")
	}
}
