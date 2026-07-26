package enum

// UserStatus 类型.
type UserStatus int

// UserStatusPermanentlyBanned 及相关导出常量.
const (
	// UserStatusActive 常量.
	UserStatusActive UserStatus = iota + 1
	// UserStatusBanned 常量.
	UserStatusBanned = 2 // 封禁中
	// UserStatusPermanentlyBanned 常量.
	UserStatusPermanentlyBanned = 3 // 永久封禁
)
