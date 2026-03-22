package rbac_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/rbac"
	domain "f_ccg_video_back_ddd/internal/domain/rbac"

	"go.uber.org/zap"
)

type selectionRoleRepository struct{ domain.RoleRepository }

func (selectionRoleRepository) FindByID(context.Context, uint) (*domain.Role, error) {
	return domain.NewRole("child", ""), nil
}

type selectionEnforcer struct{ app.Enforcer }

func (selectionEnforcer) GetRolesForUser(string) ([]string, error) {
	return []string{" middle ", "other", "middle", ""}, nil
}

func (selectionEnforcer) GetImplicitRolesForUser(role string) ([]string, error) {
	switch role {
	case "child":
		return []string{"child", " root ", "middle", "root", "other", "lost", ""}, nil
	case "middle":
		return []string{"root"}, nil
	default:
		return nil, errors.New("ancestor explanation unavailable")
	}
}

func TestInheritListPreservesSelectionOrderAndKeepsUnexplainedAncestors(t *testing.T) {
	enforcer := selectionEnforcer{}
	service := app.NewInheritService(selectionRoleRepository{}, nil, enforcer, app.NewRoleGraph(enforcer), zap.NewNop())
	got, err := service.List(t.Context(), 1)
	want := app.RoleInheritsResp{
		Direct: []string{"middle", "other"},
		Indirect: []app.RoleIndirectInheritItem{
			{Name: "root", Via: "middle"}, {Name: "lost"},
		},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("selection/downgrade changed: got=%+v err=%v want=%+v", got, err, want)
	}
}
