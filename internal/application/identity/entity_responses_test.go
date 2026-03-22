package identity

import (
	"encoding/json"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"strings"
	"testing"
)

func TestUserAndBanResponseCompatibility(t *testing.T) {
	user := identitydomain.RestoreUser(identitydomain.UserSnapshot{ID: 7, Password: "secret-password", RefreshTokenHash: "secret-refresh"})
	raw, err := json.Marshal(ToUserDTO(user))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "secret") || strings.Contains(text, "mustChangePassword") || !strings.Contains(text, `"birthday":null`) || !strings.Contains(text, `"currentRoleID":0`) {
		t.Fatal(text)
	}
	ban := identitydomain.RestoreBanRecord(identitydomain.BanRecordSnapshot{ID: 9, UserID: 7})
	raw, err = json.Marshal(ToBanRecordDTO(ban))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "endAt") || !strings.Contains(string(raw), `"userId":7`) || !strings.Contains(string(raw), `"isUnbanned":false`) {
		t.Fatal(string(raw))
	}
	if ToUserDTO(nil) != nil || ToBanRecordDTO(nil) != nil {
		t.Fatal("nil response changed")
	}
}
