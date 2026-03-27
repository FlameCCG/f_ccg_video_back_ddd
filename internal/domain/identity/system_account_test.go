package identity

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"testing"
)

func TestProvisionAdministratorPreservesAccountState(t *testing.T) {
	u := RestoreUser(UserSnapshot{ID: 7, Username: "existing", Email: "bound@example.com", Password: "old", CoinCount: -1.2, Status: enum.UserStatusBanned})
	u.ProvisionAdministrator("next", 9)
	if u.Password() != "next" || u.CurrentRoleID() != 9 || u.Level() != 6 || u.Exp() != 20000 {
		t.Fatalf("incomplete credential/experience transition: %+v", u.Snapshot())
	}
	if u.CoinCount() != -1.2 || u.Status() != enum.UserStatusBanned || u.ID() != 7 || u.Email() != "bound@example.com" || u.Username() != "existing" {
		t.Fatal("initialization changed unrelated account state")
	}
	created := NewSystemUser("initial", "hash", 9)
	if created.ID() != 0 || created.Email() != "initial@example.com" || created.RegisterSource() != "system_flag" {
		t.Fatalf("new system account contract changed: %+v", created.Snapshot())
	}
}
