package storage

import (
	"strings"
	"testing"
)

func TestMinIOChunkPrefixExcludesSiblingHash(t *testing.T) {
	t.Parallel()

	store := &MinIOStorage{chunkPrefix: defaultChunkPrefix}
	prefix := store.chunkPrefixPath("abc")
	own := "chunks/abc/0.part"
	sibling := "chunks/abcd/0.part"

	if !strings.HasSuffix(prefix, "/") {
		t.Fatalf("prefix = %q, want a directory prefix", prefix)
	}
	if !strings.HasPrefix(own, prefix) {
		t.Fatalf("prefix %q does not match %q", prefix, own)
	}
	if strings.HasPrefix(sibling, prefix) {
		t.Fatalf("prefix %q also matches %q", prefix, sibling)
	}

	index, ok := partIndexInHashDir(own, "abc")
	if !ok || index != "0" {
		t.Fatalf("partIndexInHashDir(%q) = %q, %t", own, index, ok)
	}
	if _, ok = partIndexInHashDir(sibling, "abc"); ok {
		t.Fatalf("sibling key %q was counted for abc", sibling)
	}

	var total, current int64
	for _, object := range []struct {
		key  string
		size int64
	}{
		{key: own, size: 5},
		{key: sibling, size: 30},
	} {
		if !strings.HasPrefix(object.key, prefix) {
			continue
		}

		partIndex, counted := partIndexInHashDir(object.key, "abc")
		if !counted {
			continue
		}

		total += object.size
		if partIndex == "0" {
			current = object.size
		}
	}

	if total != 5 || current != 5 {
		t.Fatalf("total=%d current=%d, want 5 and 5", total, current)
	}
}
