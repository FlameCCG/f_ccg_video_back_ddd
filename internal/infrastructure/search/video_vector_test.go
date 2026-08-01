package search

import (
	"reflect"
	"testing"
)

func TestFuseByRRFPrefersAgreementAcrossRecalls(t *testing.T) {
	t.Parallel()

	vectorHits := []vectorHit{{VideoID: 1, Score: 0.98}, {VideoID: 7, Score: 0.71}, {VideoID: 3, Score: 0.66}}
	keywordHits := []vectorHit{{VideoID: 7, Score: 42}, {VideoID: 9, Score: 31}, {VideoID: 1, Score: 8}}

	got := fuseByRRF(defaultRRFRankConstant, 4, vectorHits, keywordHits)
	want := []uint{7, 1, 9, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fuseByRRF() = %v, want %v", got, want)
	}
}

func TestFuseByRRFIgnoresRawScoreMagnitude(t *testing.T) {
	t.Parallel()

	vectorHits := []vectorHit{{VideoID: 5, Score: 0.9}}
	keywordHits := []vectorHit{{VideoID: 8, Score: 120}, {VideoID: 5, Score: 110}}

	got := fuseByRRF(defaultRRFRankConstant, 2, vectorHits, keywordHits)
	if got[0] != 5 {
		t.Fatalf("fuseByRRF() = %v, want vector leader 5 first", got)
	}
}

func TestFuseByRRFRespectsLimit(t *testing.T) {
	t.Parallel()

	hits := []vectorHit{{VideoID: 1}, {VideoID: 2}, {VideoID: 3}, {VideoID: 4}}
	if got := fuseByRRF(0, 2, hits); len(got) != 2 {
		t.Fatalf("fuseByRRF() = %v, want 2 items", got)
	}
}

func TestTopVectorHitIDsDeduplicates(t *testing.T) {
	t.Parallel()

	hits := []vectorHit{{VideoID: 3}, {VideoID: 3}, {VideoID: 4}}
	got := topVectorHitIDs(hits, 10)
	if !reflect.DeepEqual(got, []uint{3, 4}) {
		t.Fatalf("topVectorHitIDs() = %v, want [3 4]", got)
	}
}

func TestBuildVideoVectorIndexSpecUsesConfiguredDims(t *testing.T) {
	t.Parallel()

	spec := buildVideoVectorIndexSpec(1024)
	mappings, _ := spec["mappings"].(map[string]any)
	properties, _ := mappings["properties"].(map[string]any)
	embedding, _ := properties[vectorFieldEmbedding].(map[string]any)

	if embedding["dims"] != 1024 {
		t.Fatalf("dims = %v, want 1024", embedding["dims"])
	}
	if embedding["similarity"] != "cosine" {
		t.Fatalf("similarity = %v, want cosine", embedding["similarity"])
	}
}

func TestBuildVideoVectorIndexSpecFallsBackToDefaultDims(t *testing.T) {
	t.Parallel()

	spec := buildVideoVectorIndexSpec(0)
	mappings, _ := spec["mappings"].(map[string]any)
	properties, _ := mappings["properties"].(map[string]any)
	embedding, _ := properties[vectorFieldEmbedding].(map[string]any)

	if embedding["dims"] != defaultEmbeddingDims {
		t.Fatalf("dims = %v, want %d", embedding["dims"], defaultEmbeddingDims)
	}
}
