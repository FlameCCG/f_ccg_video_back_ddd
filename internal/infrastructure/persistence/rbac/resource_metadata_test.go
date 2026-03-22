package rbac_test

import (
	domain "f_ccg_video_back_ddd/internal/domain/rbac"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"testing"
)

func TestResourceMetadataReconcilesLegacyStorageWithoutUnnecessaryUpdates(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	resource, err := f.resources.Create(ctx, &domain.ApiResource{Path: "/test", Method: "GET", Summary: "test", Tags: []string{"rbac"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec(`UPDATE api_resources SET tags = ? WHERE id = ?`, `[ "rbac" ]`, resource.ID).Error; err != nil {
		t.Fatal(err)
	}
	change := domain.ResourceMetadataChange{Tags: []string{"rbac"}}
	if changed, err := f.resources.UpdateMetadata(ctx, resource.ID, change); err != nil || changed {
		t.Fatal("JSON whitespace must not trigger writes", changed, err)
	}
	var row po.ApiResource
	if err := f.db.First(&row, resource.ID).Error; err != nil {
		t.Fatal(err)
	}
	if string(row.Tags) != `[ "rbac" ]` {
		t.Fatal("unchanged storage was rewritten", string(row.Tags))
	}
	for _, raw := range []string{"broken", "null", "[]"} {
		if err := f.db.Exec(`UPDATE api_resources SET tags = ? WHERE id = ?`, raw, resource.ID).Error; err != nil {
			t.Fatal(err)
		}
		if changed, err := f.resources.UpdateMetadata(ctx, resource.ID, domain.ResourceMetadataChange{}); err != nil || !changed {
			t.Fatal("legacy empty/invalid JSON must be reconciled", raw, changed, err)
		}
		if changed, err := f.resources.UpdateMetadata(ctx, resource.ID, domain.ResourceMetadataChange{}); err != nil || changed {
			t.Fatal("reconciliation must be idempotent", raw, changed, err)
		}
	}
}
