package notification

type PagePolicy struct{}

func (PagePolicy) Normalize(page, size int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}
	return page, size
}
