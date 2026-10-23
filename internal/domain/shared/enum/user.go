package enum

// UserStatus 标识账号可用、临时封禁和永久封禁状态；解封时间事实另存于封禁记录。
type UserStatus int

const (
	// UserStatusActive 表示正常账号状态。
	UserStatusActive UserStatus = iota + 1
	// UserStatusBanned 表示处于临时封禁中，是否到期需结合封禁记录判断。
	UserStatusBanned = 2 // 封禁中
	// UserStatusPermanentlyBanned 表示永久封禁，不能由到期解封任务自动恢复。
	UserStatusPermanentlyBanned = 3 // 永久封禁
)
