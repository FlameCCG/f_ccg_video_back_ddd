package video

import (
	"testing"
	"time"
)

func TestVideoAndReportSnapshotsDoNotAliasState(t *testing.T) {
	approved := time.Now()
	v := RestoreVideo(VideoSnapshot{ID: 9, Title: "historical", ApprovedAt: &approved})
	expected := approved
	approved = approved.Add(time.Hour)
	*v.ApprovedAt() = approved
	snapshot := v.Snapshot()
	*snapshot.ApprovedAt = approved
	if !v.ApprovedAt().Equal(expected) {
		t.Fatal("video approval time escaped encapsulation")
	}
	images := []string{"original"}
	handled := time.Now()
	r := RestoreVideoReport(VideoReportSnapshot{ID: 2, ImageURLs: images, HandledAt: &handled})
	images[0] = "input"
	r.ImageURLs()[0] = "read"
	reportSnapshot := r.Snapshot()
	reportSnapshot.ImageURLs[0] = "snapshot"
	*reportSnapshot.HandledAt = handled.Add(time.Hour)
	if r.ImageURLs()[0] != "original" || !r.HandledAt().Equal(handled) {
		t.Fatal("report snapshot escaped encapsulation")
	}
	folder := RestoreFavoriteFolder(FavoriteFolderSnapshot{ID: 3, UserID: 7, Name: "original", IsDefault: true})
	if err := folder.Rename(""); err == nil || folder.Name() != "original" || folder.UserID() != 7 {
		t.Fatal("failed empty-name rename changed state")
	}
}
