package identity

import (
	"testing"
	"time"
)

func TestHistoricalIdentityStateAndCopies(t *testing.T) {
	birthday := time.Date(1990, 1, 2, 0, 0, 0, 0, time.UTC)
	user := RestoreUser(UserSnapshot{ID: 7, Username: "", Birthday: &birthday, CoinCount: -1.2})
	birthday = birthday.AddDate(1, 0, 0)
	if user.ID() != 7 || user.Birthday().Year() != 1990 || user.CoinCount() != -1.2 {
		t.Fatal("restore must preserve history and copy its input")
	}
	exposed := user.Birthday()
	*exposed = exposed.AddDate(1, 0, 0)
	snapshot := user.Snapshot()
	*snapshot.Birthday = snapshot.Birthday.AddDate(2, 0, 0)
	if user.Birthday().Year() != 1990 {
		t.Fatal("birthday read or snapshot aliases internal state")
	}
	tags := []string{"one"}
	conf := RestoreUserConf(UserConfSnapshot{UserID: 7, LikeTags: tags})
	tags[0] = "input"
	conf.LikeTags()[0] = "read"
	snapshotConf := conf.Snapshot()
	snapshotConf.LikeTags[0] = "snapshot"
	if conf.LikeTags()[0] != "one" {
		t.Fatal("configuration tags escape encapsulation")
	}
	end := time.Now()
	ban := RestoreBanRecord(BanRecordSnapshot{ID: 3, EndAt: &end})
	expected := end
	*ban.EndAt() = end.Add(time.Hour)
	stored := ban.Snapshot()
	*stored.EndAt = end.Add(2 * time.Hour)
	if !ban.EndAt().Equal(expected) {
		t.Fatal("ban end time escapes encapsulation")
	}
}

func TestUsernamePolicyUsesSuppliedFacts(t *testing.T) {
	now := time.Unix(0, 12345)
	if got := (UsernamePolicy{}).Generate("12345678-abcd", now); got != UsernamePrefix+"_123456782345" {
		t.Fatal(got)
	}
}
