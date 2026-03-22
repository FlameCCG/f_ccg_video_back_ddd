package search

import "sort"

const DefaultRRFRankConstant = 60

type RankFusion struct{}

// FuseRRF 累计各召回来源的倒数名次分。同分时保留首次召回顺序；非正上限不截断。
func (RankFusion) FuseRRF(rankConstant, limit int, rankings ...[]uint) []uint {
	if rankConstant <= 0 {
		rankConstant = DefaultRRFRankConstant
	}
	scores := make(map[uint]float64)
	order := make([]uint, 0)
	for _, ranking := range rankings {
		for rank, id := range ranking {
			if _, seen := scores[id]; !seen {
				order = append(order, id)
			}
			scores[id] += 1 / float64(rankConstant+rank+1)
		}
	}
	position := make(map[uint]int, len(order))
	for index, id := range order {
		position[id] = index
	}
	sort.SliceStable(order, func(a, b int) bool {
		left, right := order[a], order[b]
		if scores[left] != scores[right] {
			return scores[left] > scores[right]
		}
		return position[left] < position[right]
	})
	if limit > 0 && len(order) > limit {
		order = order[:limit]
	}
	return order
}

func (RankFusion) TopIDs(ranking []uint, limit int) []uint {
	ids := make([]uint, 0, len(ranking))
	seen := make(map[uint]struct{}, len(ranking))
	for _, id := range ranking {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
		if limit > 0 && len(ids) >= limit {
			break
		}
	}
	return ids
}
