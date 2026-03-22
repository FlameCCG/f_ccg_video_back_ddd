package identity

// ActivityListingPolicy 维护身份入口的分页和排序取值，不依赖其它上下文。
type ActivityListingPolicy struct{}

func (ActivityListingPolicy) UserVideoPage(page, size int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	return page, size
}
func (p ActivityListingPolicy) SearchPage(page, size int) (int, int) {
	page, size = p.UserVideoPage(page, size)
	if size > 100 {
		size = 100
	}
	return page, size
}
func (p ActivityListingPolicy) MentionPage(page, size int) (int, int) {
	page, size = p.UserVideoPage(page, size)
	if size > 50 {
		size = 50
	}
	return page, size
}
func (ActivityListingPolicy) UserSort(value int) int {
	switch value {
	case 0, 1, 2, 3:
		return value
	default:
		return 1
	}
}
func (ActivityListingPolicy) SortOrder(value int) int {
	if value == 1 {
		return 1
	}
	return 0
}
