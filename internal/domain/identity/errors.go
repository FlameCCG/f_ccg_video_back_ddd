package identity

import "errors"

// 领域层通用错误（文案与原实现保持一致，保证接口错误信息不变）。
var (
	// ErrUserNotLogin 用户未登录（userID 为 0）.
	ErrUserNotLogin = errors.New("用户未登录")
	// ErrUserNotFound 用户不存在.
	ErrUserNotFound = errors.New("用户不存在")
	// ErrUserIDRequired 用户 ID 不能为空.
	ErrUserIDRequired = errors.New("用户ID不能为空")
	// ErrIPRequired 登录记录必须带 IP.
	ErrIPRequired = errors.New("ip 不能为空")
	// ErrUserRecordNotFound 数据库中不存在该用户记录。
	//
	// 原实现直接把 gorm.ErrRecordNotFound 透传给中间件用于区分「用户不存在」；
	// 新架构由领域层提供哨兵错误，避免 interfaces 层依赖 GORM。
	ErrUserRecordNotFound = errors.New("user record not found")
	// ErrDuplicateRecord 幂等键冲突：本次流水已存在，调用方应视为「未生效」而非错误。
	ErrDuplicateRecord = errors.New("duplicate record")
)
