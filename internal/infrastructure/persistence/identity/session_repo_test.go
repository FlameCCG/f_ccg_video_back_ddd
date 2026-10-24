package identity

import (
	app "f_ccg_video_back_ddd/internal/application/identity"
	"testing"
)

func TestRefreshDigestCASAndPasswordUpdate(t *testing.T) {
	db := openUserProfileTestDB(t)
	repo := NewUserRepository(db, nil)
	if ok, err := repo.SetRefreshToken(t.Context(), 1, "password-hash", "original"); err != nil || !ok {
		t.Fatalf("set ok=%t err=%v", ok, err)
	}
	if ok, err := repo.RotateRefreshToken(t.Context(), 1, "password-hash", "original", "next"); err != nil || !ok {
		t.Fatalf("rotate ok=%t err=%v", ok, err)
	}
	if ok, err := repo.RotateRefreshToken(t.Context(), 1, "password-hash", "original", "replay"); err != nil || ok {
		t.Fatalf("replay ok=%t err=%v", ok, err)
	}
	if err := db.Exec("UPDATE users SET must_change_password = 1 WHERE id = 1").Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdatePassword(t.Context(), 1, "new-password-hash"); err != nil {
		t.Fatal(err)
	}
	user, err := repo.FindByID(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if user.RefreshTokenHash != "" || user.MustChangePassword || user.Password != "new-password-hash" {
		t.Fatalf("credential state not reset: %+v", user)
	}
	if ok, err := repo.SetRefreshToken(t.Context(), 1, "password-hash", "stale-login"); err != nil || ok {
		t.Fatalf("stale login race ok=%t err=%v", ok, err)
	}
}
func TestAdminUserRoleFilterPreservesAllUserFields(t *testing.T) {
	db := openUserProfileTestDB(t)
	for _, sql := range []string{`CREATE TABLE roles (id INTEGER PRIMARY KEY,name TEXT)`, `CREATE TABLE user_roles (user_id INTEGER,role_id INTEGER)`, `INSERT INTO roles(id,name) VALUES (3,'editor')`, `INSERT INTO user_roles(user_id,role_id) VALUES (1,3)`} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows, total, err := NewAdminUserQuery(db).List(t.Context(), app.AdminUserListParams{Page: 1, PageSize: 20, RoleID: 3})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("total=%d rows=%v", total, rows)
	}
	if rows[0].Username != "joined-user" || rows[0].Email != "joined-user@example.com" || rows[0].Avatar != "avatar.png" {
		t.Fatalf("role filter lost user fields: %+v", rows[0])
	}
	if len(rows[0].RoleNames) != 1 || rows[0].RoleNames[0] != "editor" {
		t.Fatalf("role preload lost: %+v", rows[0])
	}
}
