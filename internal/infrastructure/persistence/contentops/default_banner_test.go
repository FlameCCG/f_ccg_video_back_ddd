package contentops

import (
	"context"
	"errors"
	app "f_ccg_video_back_ddd/internal/application/contentops"
	contentopsdomain "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"testing"
)

func TestDefaultBannerPreservesFieldsAndClearsNonHomePartition(t *testing.T) {
	repo, _, db := newBannerFixture(t)
	row := po.Banner{Cover: "same-cover", Href: "keep-target", Type: enum.BannerTypeHome, PartitionID: 42}
	seedBanner(t, db, &row, false)
	s := app.NewDefaultBannerService(gormx.NewTxManager(db), repo)
	created, err := s.UpsertBannerByCover(t.Context(), row.Cover, enum.BannerTypeUser)
	if err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	var saved po.Banner
	if err := db.First(&saved, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Type != enum.BannerTypeUser || saved.PartitionID != 0 || !saved.Show || saved.Href != "keep-target" || saved.Cover != row.Cover {
		t.Fatalf("invalid placement or unrelated field overwritten: %+v", saved)
	}
	created, err = s.UpsertBannerByCover(t.Context(), "new-cover", enum.BannerTypeHome)
	if err != nil || !created {
		t.Fatalf("new banner created=%v err=%v", created, err)
	}
	created, err = s.UpsertBannerByCover(t.Context(), "new-cover", enum.BannerTypeHome)
	if err != nil || created {
		t.Fatalf("replay created=%v err=%v", created, err)
	}
}

type failDefaultBannerUpdate struct {
	contentopsdomain.DefaultBannerRepository
}

func (r failDefaultBannerUpdate) Update(ctx context.Context, id uint, change contentopsdomain.BannerChanges) error {
	if err := r.DefaultBannerRepository.Update(ctx, id, change); err != nil {
		return err
	}
	return errors.New("save failed")
}

func TestDefaultBannerFailureRollsBackRelatedFields(t *testing.T) {
	repo, _, db := newBannerFixture(t)
	row := po.Banner{Cover: "original", Type: enum.BannerTypeHome, PartitionID: 42}
	seedBanner(t, db, &row, false)
	s := app.NewDefaultBannerService(gormx.NewTxManager(db), failDefaultBannerUpdate{repo})
	if _, err := s.UpsertBannerByCover(t.Context(), row.Cover, enum.BannerTypeUser); err == nil {
		t.Fatal("expected rollback")
	}
	var saved po.Banner
	if err := db.First(&saved, row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Type != enum.BannerTypeHome || saved.PartitionID != 42 || saved.Show {
		t.Fatalf("failed action leaked placement: %+v", saved)
	}
}
