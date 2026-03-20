package notification

func normalizePaging(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	return page, pageSize
}
