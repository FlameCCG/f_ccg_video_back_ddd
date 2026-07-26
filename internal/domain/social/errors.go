package social

import "errors"

// 领域层错误（文案与原 api/social_api、service/user_service/focus_service.go 逐字一致，
// 保证接口错误信息不变）。
var (
	// ErrUserIDRequired 用户 ID 不能为空.
	ErrUserIDRequired = errors.New("用户ID不能为空")
	// ErrUserNotFound 用户不存在.
	ErrUserNotFound = errors.New("用户不存在")
	// ErrCannotFocusSelf 不能关注自己.
	ErrCannotFocusSelf = errors.New("不能关注自己")
	// ErrTargetUserNotFound 目标用户不存在.
	ErrTargetUserNotFound = errors.New("目标用户不存在")
	// ErrAlreadyFocused 已经关注过该用户.
	ErrAlreadyFocused = errors.New("已经关注过该用户")
	// ErrFocusFailed 关注写入失败（底层错误已记录日志，仅对外暴露统一文案）.
	ErrFocusFailed = errors.New("关注失败")
	// ErrNotFocused 未关注该用户（取消关注时无匹配记录）.
	ErrNotFocused = errors.New("未关注该用户")
	// ErrUnfocusFailed 取消关注写入失败.
	ErrUnfocusFailed = errors.New("取消关注失败")
	// ErrFollowNotPublic 用户未公开关注列表.
	ErrFollowNotPublic = errors.New("用户未公开关注")
	// ErrFansNotPublic 用户未公开粉丝列表.
	ErrFansNotPublic = errors.New("用户未公开粉丝")
	// ErrForeignPageLimited 非本人查看列表时只能翻前若干页.
	ErrForeignPageLimited = errors.New("只能查询前5页")
	// ErrSelfRelation 查询与自己的关注关系无意义.
	ErrSelfRelation = errors.New("你查看的是自己，没有关注关系")
	// ErrListQueryFailed 列表查询失败（对外统一文案）.
	ErrListQueryFailed = errors.New("查询失败")
)
