package domainerr

import "errors"

// 领域层通用错误。
var (
	// ErrUserNotLogin 用户未登录（userID 为 0）.
	ErrUserNotLogin = errors.New("用户未登录")
	// ErrUserNotFound 用户不存在.
	ErrUserNotFound = errors.New("用户不存在")
	// ErrUserIDRequired 用户 ID 不能为空.
	ErrUserIDRequired = errors.New("用户ID不能为空")
	// ErrIPRequired 登录记录必须带 IP.
	ErrIPRequired = errors.New("ip 不能为空")
	// ErrUserRecordNotFound 表示用户记录不存在，由持久化层映射为领域哨兵，接口层无需依赖 GORM。
	ErrUserRecordNotFound = errors.New("user record not found")
	// ErrDuplicateRecord 幂等键冲突：本次流水已存在，调用方应视为「未生效」而非错误。
	ErrDuplicateRecord = errors.New("duplicate record")
)
