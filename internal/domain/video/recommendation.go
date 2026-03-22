package video

// RelatedSelection 逐来源接收候选，保持来源顺序、排除当前作品并维护查询排除集合。
// 按一次推荐请求创建，不放入共享服务。
type RelatedSelection struct {
	limit int
	ids   []uint
	seen  map[uint]struct{}
}

func NewRelatedSelection(videoID uint, requested int) *RelatedSelection {
	limit := (ListingPolicy{}).RelatedSize(requested)
	return &RelatedSelection{limit: limit, ids: make([]uint, 0, limit), seen: map[uint]struct{}{videoID: {}}}
}

func (s *RelatedSelection) Add(candidates []uint) {
	for _, id := range candidates {
		if !s.NeedsMore() {
			return
		}
		if _, exists := s.seen[id]; exists {
			continue
		}
		s.seen[id] = struct{}{}
		s.ids = append(s.ids, id)
	}
}
func (s *RelatedSelection) NeedsMore() bool { return len(s.ids) < s.limit }
func (s *RelatedSelection) FetchLimit() int { return s.limit * 2 }
func (s *RelatedSelection) IDs() []uint     { return append([]uint{}, s.ids...) }
func (s *RelatedSelection) ExcludedIDs() []uint {
	ids := make([]uint, 0, len(s.seen))
	for id := range s.seen {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}
