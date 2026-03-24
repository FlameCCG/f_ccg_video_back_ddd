package rbac

import (
	"context"
	"errors"
	"testing"

	rbacdomain "f_ccg_video_back_ddd/internal/domain/rbac"
)

type creatorRoleFacts struct {
	rbacdomain.UserRoleRepository
	roleID uint
	found  bool
	err    error
	reads  int
}

func (r *creatorRoleFacts) CurrentRoleID(context.Context, uint) (uint, bool, error) {
	r.reads++
	return r.roleID, r.found, r.err
}

type creatorRoleRepository struct {
	rbacdomain.RoleRepository
	role  *rbacdomain.Role
	err   error
	reads int
}

func (r *creatorRoleRepository) FindIDByID(context.Context, uint) (*rbacdomain.Role, error) {
	r.reads++
	return r.role, r.err
}

type creatorMenuGrants struct {
	rbacdomain.RoleMenuRepository
	err            error
	roleID, menuID uint
}

func (r *creatorMenuGrants) GrantDirect(_ context.Context, roleID, menuID uint) error {
	r.roleID, r.menuID = roleID, menuID
	return r.err
}

func TestMenuCreatorGrantPreservesQualificationAndErrorOrder(t *testing.T) {
	cause := errors.New("storage unavailable")
	for _, tc := range []struct {
		name                       string
		creatorID, roleID          uint
		found, roleExists          bool
		userErr, roleErr, grantErr error
		want                       string
		userReads, roleReads       int
	}{
		{name: "missing creator", want: rbacdomain.ErrMenuCreatorRoleMissing.Error()},
		{name: "user read fails", creatorID: 7, userErr: cause, want: "读取创建者角色失败: storage unavailable", userReads: 1},
		{name: "missing user", creatorID: 7, roleID: 9, want: rbacdomain.ErrMenuCreatorRoleMissing.Error(), userReads: 1},
		{name: "zero role", creatorID: 7, found: true, want: rbacdomain.ErrMenuCreatorRoleMissing.Error(), userReads: 1},
		{name: "role read fails", creatorID: 7, found: true, roleID: 9, roleErr: cause, want: "读取创建者当前角色失败: storage unavailable", userReads: 1, roleReads: 1},
		{name: "missing role", creatorID: 7, found: true, roleID: 9, want: rbacdomain.ErrMenuCreatorRoleMissing.Error(), userReads: 1, roleReads: 1},
		{name: "grant fails", creatorID: 7, found: true, roleID: 9, roleExists: true, grantErr: cause, want: "授予创建者菜单权限失败: storage unavailable", userReads: 1, roleReads: 1},
		{name: "valid grant", creatorID: 7, found: true, roleID: 9, roleExists: true, userReads: 1, roleReads: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := &creatorRoleFacts{roleID: tc.roleID, found: tc.found, err: tc.userErr}
			roles := &creatorRoleRepository{err: tc.roleErr}
			if tc.roleExists {
				roles.role = rbacdomain.RestoreRole(rbacdomain.RoleSnapshot{ID: tc.roleID})
			}
			grants := &creatorMenuGrants{err: tc.grantErr}
			service := &MenuService{userRoles: facts, roles: roles, roleMenus: grants}
			err := service.grantMenuToCreatorRole(t.Context(), tc.creatorID, 12)
			if tc.want == "" {
				if err != nil || grants.roleID != 9 || grants.menuID != 12 {
					t.Fatalf("grant=%+v err=%v", grants, err)
				}
			} else if err == nil || err.Error() != tc.want {
				t.Fatalf("error=%v want=%q", err, tc.want)
			}
			if (tc.userErr != nil || tc.roleErr != nil || tc.grantErr != nil) && !errors.Is(err, cause) {
				t.Fatalf("lost underlying error: %v", err)
			}
			if facts.reads != tc.userReads || roles.reads != tc.roleReads {
				t.Fatalf("qualification/query order changed: user=%d role=%d", facts.reads, roles.reads)
			}
		})
	}
}
