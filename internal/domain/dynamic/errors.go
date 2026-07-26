package dynamic

import "errors"

// 领域层错误（文案与原 service/dynamic_service 逐字一致，保证接口错误信息不变）。
var (
	// ErrUserNotLogin 用户未登录（userID 为 0）.
	ErrUserNotLogin = errors.New("用户未登录")
	// ErrLoginRequired 点赞路径的未登录文案（原 ToggleLike 使用「请先登录」）.
	ErrLoginRequired = errors.New("请先登录")
	// ErrContentEmpty 动态正文不能为空.
	ErrContentEmpty = errors.New("内容不能为空")
	// ErrContentTooLong 动态正文超长.
	ErrContentTooLong = errors.New("内容过长")
	// ErrDynamicIDRequired dynamicId 不能为空.
	ErrDynamicIDRequired = errors.New("dynamicId 不能为空")
	// ErrDynamicIDsRequired 批量删除时 dynamicIds 不能为空.
	ErrDynamicIDsRequired = errors.New("dynamicIds 不能为空")
	// ErrVideoIDRequired videoId 不能为空.
	ErrVideoIDRequired = errors.New("videoId 不能为空")
	// ErrAuthorIDRequired authorId 不能为空.
	ErrAuthorIDRequired = errors.New("authorId 不能为空")
	// ErrUserIDRequired 作品流必须指定作者 userId.
	ErrUserIDRequired = errors.New("userId 不能为空")
	// ErrDynamicNotFound 动态不存在.
	ErrDynamicNotFound = errors.New("动态不存在")
	// ErrDeleteForbidden 无权删除该动态.
	ErrDeleteForbidden = errors.New("无权删除该动态")
	// ErrPinForbidden 无权操作该动态.
	ErrPinForbidden = errors.New("无权操作该动态")
	// ErrVideoPinForbidden 无权操作该视频.
	ErrVideoPinForbidden = errors.New("无权操作该视频")
	// ErrUserNotFound 用户不存在.
	ErrUserNotFound = errors.New("用户不存在")
	// ErrLikeDuplicated 点赞记录已存在（唯一键冲突），调用方据此切换为取消点赞.
	ErrLikeDuplicated = errors.New("dynamic like duplicated")
	// ErrRecordNotFound 记录不存在。
	//
	// 原实现在若干路径（Create/PinVideo/ListFeed 读取作者）上把 gorm.ErrRecordNotFound
	// 原样透传给响应，错误文案为 "record not found"。领域层不依赖 GORM，
	// 故在此定义同文案哨兵错误，保证响应体与原实现逐字一致。
	ErrRecordNotFound = errors.New("record not found")
)
