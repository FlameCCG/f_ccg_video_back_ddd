package identity

import "testing"

func TestActivityListingKeepsEndpointSpecificBounds(t *testing.T) {
	p := ActivityListingPolicy{}
	if page, size := p.UserVideoPage(0, 200); page != 1 || size != 200 {
		t.Fatal(page, size)
	}
	if page, size := p.SearchPage(-1, 200); page != 1 || size != 100 {
		t.Fatal(page, size)
	}
	if page, size := p.MentionPage(0, 200); page != 1 || size != 50 {
		t.Fatal(page, size)
	}
	if page, size := p.SearchPage(2, 0); page != 2 || size != 20 {
		t.Fatal(page, size)
	}
	if p.UserSort(-1) != 1 || p.UserSort(0) != 0 || p.UserSort(3) != 3 || p.SortOrder(1) != 1 || p.SortOrder(2) != 0 {
		t.Fatal("sort contract changed")
	}
}
