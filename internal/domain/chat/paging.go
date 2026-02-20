package chat

const (
	// DefaultConversationPageSize 会话列表默认每页数量。
	DefaultConversationPageSize = 20
	// MaxConversationPageSize 会话列表每页数量上限（服务端二次 clamp）。
	MaxConversationPageSize = 50
	// MessagePageSize 消息列表固定每页返回条数。
	MessagePageSize = 10
	// MessageFetchLimit 消息列表实际拉取条数：多取一条用于判断 hasMore。
	MessageFetchLimit = MessagePageSize + 1
	// messageCursorLen before 游标（ObjectID 十六进制）的固定长度。
	messageCursorLen = 24
)

// NormalizeConversationPage 规范化分页参数：默认 page=1、pageSize=20，并截断过大 pageSize。
func NormalizeConversationPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}

	if pageSize <= 0 {
		pageSize = DefaultConversationPageSize
	}

	if pageSize > MaxConversationPageSize {
		pageSize = MaxConversationPageSize
	}

	return page, pageSize
}

// ValidateMessageCursor 校验 before 游标：必须是 24 位十六进制字符。
//
// 判定与 Mongo 驱动的 ObjectIDFromHex 完全等价（先查长度再查十六进制字符集），
// 空串表示不带游标、从最新一条开始。
func ValidateMessageCursor(before string) error {
	if before == "" {
		return nil
	}

	if len(before) != messageCursorLen {
		return ErrInvalidCursor
	}

	for i := range len(before) {
		if !isHexDigit(before[i]) {
			return ErrInvalidCursor
		}
	}

	return nil
}

func isHexDigit(c byte) bool {
	switch {
	case c >= '0' && c <= '9':
		return true
	case c >= 'a' && c <= 'f':
		return true
	case c >= 'A' && c <= 'F':
		return true
	default:
		return false
	}
}

// TrimMessagePage 把「多取一条」的降序结果整理为一页倒序（旧→新）消息。
//
// 返回值分别为该页消息与是否还有更早的消息。
func TrimMessagePage[T any](list []T) ([]T, bool) {
	hasMore := false
	if len(list) > MessagePageSize {
		hasMore = true
		list = list[:MessagePageSize]
	}

	// 返回倒序（旧->新）更适合聊天展示
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}

	return list, hasMore
}
