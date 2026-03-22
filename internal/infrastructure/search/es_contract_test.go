package search

import "testing"

func TestEscapeESWildcard(t *testing.T) {
	got, ok := (esContract{}).EscapeWildcard("ce6*")
	if !ok {
		t.Fatal("expected ok")
	}

	if got != `ce6\*` {
		t.Fatalf("got %q", got)
	}

	if _, ok := (esContract{}).EscapeWildcard("   "); ok {
		t.Fatal("blank should be rejected")
	}
}

func TestParseDocumentID(t *testing.T) {
	tests := []struct {
		name  string
		docID string
		want  uint
		ok    bool
	}{
		{name: "with prefix", docID: "users:12", want: 12, ok: true},
		{name: "plain id", docID: "34", want: 34, ok: true},
		{name: "invalid", docID: "users:abc", want: 0, ok: false},
		{name: "empty", docID: "", want: 0, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := (esContract{}).ParseDocumentID(tt.docID)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("ParseDocumentID(%q) = (%d, %v), want (%d, %v)", tt.docID, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestDocumentIDPrefixes(t *testing.T) {
	if got := (esContract{}).VideoDocumentID(7); got != "videos:7" {
		t.Fatalf("video id = %q", got)
	}

	if got := (esContract{}).UserDocumentID(12); got != "users:12" {
		t.Fatalf("user id = %q", got)
	}
}
