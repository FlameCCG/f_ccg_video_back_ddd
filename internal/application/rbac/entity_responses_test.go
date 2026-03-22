package rbac

import (
	"encoding/json"
	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
	"strings"
	"testing"
)

func TestMenuAndRoleResponseCompatibility(t *testing.T) {
	raw, err := json.Marshal(ToMenuDTO(rbacdomain.RestoreMenu(rbacdomain.MenuSnapshot{ID: 2})))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"parentId":null`) || strings.Contains(string(raw), "children") || !strings.Contains(string(raw), `"keepAlive":false`) {
		t.Fatal(string(raw))
	}
	raw, err = json.Marshal(ToRoleDTO(rbacdomain.NewRole("role", "")))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"id":0,"name":"role","desc":""}` {
		t.Fatal(string(raw))
	}
	if ToMenuDTOs(nil) != nil || ToRoleDTOs(nil) != nil {
		t.Fatal("nil collections changed")
	}
	raw, err = json.Marshal(ToMenuDTOs([]*rbacdomain.Menu{}))
	if err != nil || string(raw) != "[]" {
		t.Fatalf("empty collection: %s %v", raw, err)
	}
}
