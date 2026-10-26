package rule

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareNormalizesCamelCaseFilterAndFieldMapping(t *testing.T) {
	t.Parallel()

	r := &Rule{
		FieldMapping: map[string]string{
			"authorUsername": "author_name",
		},
		Filter: []string{"id", "authorUsername", "createdAt", "danmuCount"},
	}

	{
		err := r.Prepare()
		require.NoError(t, err, "prepare failed: %v", err)
	}

	{
		_, ok := r.FieldMapping["author_username"]
		require.True(t, ok, "expected field mapping key to normalize to author_username, got %+v", r.FieldMapping)
	}

	want := []string{"id", "author_username", "created_at", "danmu_count"}
	require.Len(t, r.Filter, len(want), "expected %d filters, got %d: %+v", len(want), len(r.Filter), r.Filter)

	for i, item := range want {
		require.Equal(t, r.Filter[i], item, "expected filter[%d]=%s, got %s", i, item, r.Filter[i])
	}
}
