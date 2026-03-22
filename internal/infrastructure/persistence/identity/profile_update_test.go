package identity

import (
	"errors"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	video "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"
)

func TestProfileAuthorRenameUsesOneDatabaseTransaction(t *testing.T) {
	for _, failCascade := range []bool{false, true} {
		name := "success"
		if failCascade {
			name = "cascade rolls back"
		}
		t.Run(name, func(t *testing.T) {
			db := openUserProfileTestDB(t)
			for _, sql := range []string{
				`CREATE TABLE videos (id integer PRIMARY KEY, author_username text, updated_at datetime)`,
				`CREATE TABLE video_reports (id integer PRIMARY KEY, reporter_username text, updated_at datetime)`,
				`CREATE TABLE danmu_reports (id integer PRIMARY KEY, reporter_username text, updated_at datetime)`,
				`INSERT INTO videos (id, author_username) VALUES (1, 'joined-user')`,
				`INSERT INTO video_reports (id, reporter_username) VALUES (1, 'joined-user')`,
				`INSERT INTO danmu_reports (id, reporter_username) VALUES (1, 'joined-user')`,
			} {
				if err := db.Exec(sql).Error; err != nil {
					t.Fatal(err)
				}
			}
			if failCascade {
				// Fail the final cascade after the user, video and first report were updated.
				if err := db.Exec(`DROP TABLE danmu_reports`).Error; err != nil {
					t.Fatal(err)
				}
			}
			repo := NewUserRepository(db, nil)
			service := app.NewProfileService(nil, gormx.NewTxManager(db), repo, nil, nil, nil, nil, nil, video.NewAuthorRenameRepository(db), nil)
			err := service.UpdateProfile(t.Context(), app.UpdateProfileInput{UserID: 1, Username: "renamed"})
			wantName := "renamed"
			if failCascade {
				wantName = "joined-user"
				if !errors.Is(err, app.ErrUpdateUserFailed) {
					t.Fatalf("cascade error=%v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			user, err := repo.FindByID(t.Context(), 1)
			if err != nil || user.Username() != wantName || user.CoinCount() != 4.5 || user.Email() != "joined-user@example.com" {
				t.Fatalf("user=%+v error=%v", user.Snapshot(), err)
			}
			for table, column := range map[string]string{"videos": "author_username", "video_reports": "reporter_username", "danmu_reports": "reporter_username"} {
				if failCascade && table == "danmu_reports" {
					continue
				}
				var name string
				if err := db.Table(table).Select(column).Where("id = ?", 1).Scan(&name).Error; err != nil || name != wantName {
					t.Fatalf("%s name=%q, want %q, error=%v", table, name, wantName, err)
				}
			}
		})
	}
}
