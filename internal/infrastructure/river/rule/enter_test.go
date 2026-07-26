package rule

import "testing"

func TestPrepareNormalizesCamelCaseFilterAndFieldMapping(t *testing.T) {
	r := &Rule{
		FieldMapping: map[string]string{
			"authorUsername": "author_name",
		},
		Filter: []string{"id", "authorUsername", "createdAt", "danmuCount"},
	}

	if err := r.Prepare(); err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	if _, ok := r.FieldMapping["author_username"]; !ok {
		t.Fatalf("expected field mapping key to normalize to author_username, got %+v", r.FieldMapping)
	}

	want := []string{"id", "author_username", "created_at", "danmu_count"}
	if len(r.Filter) != len(want) {
		t.Fatalf("expected %d filters, got %d: %+v", len(want), len(r.Filter), r.Filter)
	}

	for i, item := range want {
		if r.Filter[i] != item {
			t.Fatalf("expected filter[%d]=%s, got %s", i, item, r.Filter[i])
		}
	}
}
