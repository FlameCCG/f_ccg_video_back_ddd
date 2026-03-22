package rbac

import "testing"

func TestResourceTagsStorageCompatibility(t *testing.T) {
	codec := resourceTags{}
	for _, tc := range []struct {
		a, b  string
		equal bool
	}{
		{"", "", true}, {`[ "rbac" ]`, `["rbac"]`, true},
		{`{"a":1,"b":2}`, `{"b":2,"a":1}`, true},
		{"broken", "broken", true}, {"broken", "", false},
		{"null", "", false}, {"[]", "", false},
	} {
		if got := codec.equal([]byte(tc.a), []byte(tc.b)); got != tc.equal {
			t.Fatalf("equal(%q,%q)=%v", tc.a, tc.b, got)
		}
	}
	if codec.encode([]string{}) != nil || codec.decode([]byte("broken")) != nil {
		t.Fatal("empty/invalid storage compatibility changed")
	}
}
