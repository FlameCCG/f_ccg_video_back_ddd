package identity

import (
	"testing"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/identity"
)

func TestRefreshDigestCASAndPasswordUpdate(t *testing.T) {
	t.Parallel()

	db := openUserProfileTestDB(t)

	repo := NewUserRepository(db, nil)
	{
		ok, err := repo.SetRefreshToken(t.Context(), 1, "password-hash", "original")
		require.NoError(t, err, "set ok=%t err=%v", ok, err)
		require.True(t, ok, "set ok=%t err=%v", ok, err)
	}

	{
		ok, err := repo.RotateRefreshToken(t.Context(), 1, "password-hash", "original", "next")
		require.NoError(t, err, "rotate ok=%t err=%v", ok, err)
		require.True(t, ok, "rotate ok=%t err=%v", ok, err)
	}

	{
		ok, err := repo.RotateRefreshToken(t.Context(), 1, "password-hash", "original", "replay")
		require.NoError(t, err, "replay ok=%t err=%v", ok, err)
		require.False(t, ok, "replay ok=%t err=%v", ok, err)
	}

	{
		err := db.Exec("UPDATE users SET must_change_password = 1 WHERE id = 1").Error
		require.NoError(t, err)
	}

	{
		err := repo.UpdatePassword(t.Context(), 1, "new-password-hash")
		require.NoError(t, err)
	}

	user, err := repo.FindByID(t.Context(), 1)
	require.NoError(t, err)

	require.Empty(t, user.RefreshTokenHash, "credential state not reset: %+v", user)
	require.False(t, user.MustChangePassword, "credential state not reset: %+v", user)
	require.Equal(t, "new-password-hash", user.Password, "credential state not reset: %+v", user)

	{
		ok, err := repo.SetRefreshToken(t.Context(), 1, "password-hash", "stale-login")
		require.NoError(t, err, "stale login race ok=%t err=%v", ok, err)
		require.False(t, ok, "stale login race ok=%t err=%v", ok, err)
	}
}

func TestAdminUserRoleFilterPreservesAllUserFields(t *testing.T) {
	t.Parallel()

	db := openUserProfileTestDB(t)

	for _, sql := range []string{`CREATE TABLE roles (id INTEGER PRIMARY KEY,name TEXT)`, `CREATE TABLE user_roles (user_id INTEGER,role_id INTEGER)`, `INSERT INTO roles(id,name) VALUES (3,'editor')`, `INSERT INTO user_roles(user_id,role_id) VALUES (1,3)`} {
		{
			err := db.Exec(sql).Error
			require.NoError(t, err)
		}
	}

	rows, total, err := NewAdminUserQuery(db).List(t.Context(), app.AdminUserListParams{Page: 1, PageSize: 20, RoleID: 3})
	require.NoError(t, err)

	require.EqualValues(t, 1, total, "total=%d rows=%v", total, rows)
	require.Len(t, rows, 1, "total=%d rows=%v", total, rows)

	require.Equal(t, "joined-user", rows[0].Username, "role filter lost user fields: %+v", rows[0])
	require.Equal(t, "joined-user@example.com", rows[0].Email, "role filter lost user fields: %+v", rows[0])
	require.Equal(t, "avatar.png", rows[0].Avatar, "role filter lost user fields: %+v", rows[0])

	require.Len(t, rows[0].RoleNames, 1, "role preload lost: %+v", rows[0])
	require.Equal(t, "editor", rows[0].RoleNames[0], "role preload lost: %+v", rows[0])
}
