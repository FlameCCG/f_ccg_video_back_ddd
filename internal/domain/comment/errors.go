package comment

import "errors"

// 评论流程的领域错误。
var (
	// ErrUserNotLogin 用户未登录.
	ErrUserNotLogin = errors.New("用户未登录")
	// ErrTargetRequired videoId 与 dynamicId 必须且只能给一个.
	ErrTargetRequired = errors.New("请传入 videoId 或 dynamicId")
	// ErrContentEmpty 无文字且无图片.
	ErrContentEmpty = errors.New("评论内容不能为空")
	// ErrContentTooLong 文字超过 1000 个字符.
	ErrContentTooLong = errors.New("评论内容过长")
	// ErrVideoNotPublished 视频不存在或未发布.
	ErrVideoNotPublished = errors.New("视频不存在或未发布")
	// ErrDynamicNotFound 动态不存在.
	ErrDynamicNotFound = errors.New("动态不存在")
	// ErrParentNotFound 被回复的评论不存在.
	ErrParentNotFound = errors.New("回复的评论不存在")
	// ErrCommentNotFound 评论不存在.
	ErrCommentNotFound = errors.New("评论不存在")
	// ErrDeleteForbidden 非评论作者且非作品作者.
	ErrDeleteForbidden = errors.New("无权删除该评论")
	// ErrPinForbidden 非作品作者.
	ErrPinForbidden = errors.New("无权置顶该评论")
	// ErrCommentIDRequired commentId 缺失.
	ErrCommentIDRequired = errors.New("commentId 不能为空")
	// ErrPinRootOnly 仅一级评论可置顶.
	ErrPinRootOnly = errors.New("仅支持置顶一级评论")
	// ErrPinTargetMissing 评论未关联视频或动态.
	ErrPinTargetMissing = errors.New("评论未关联视频或动态")
	// ErrAuthorIDRequired 创作中心列表缺少作者 ID.
	ErrAuthorIDRequired = errors.New("参数错误")
	// ErrAuthorNotFound 创作中心列表的作者不存在.
	ErrAuthorNotFound = errors.New("用户不存在")
	// ErrCommentIDsRequired 管理端批量删除缺少 commentIds.
	ErrCommentIDsRequired = errors.New("commentIds 不能为空")

	// ErrRecordNotFound 表示关联的视频或动态记录缺失。
	//
	// 持久化层将记录缺失映射为此领域哨兵，接口响应文案为 "record not found"。
	ErrRecordNotFound = errors.New("record not found")

	// ErrLikeCacheUnavailable Redis 未配置，点赞缓存不可用.
	ErrLikeCacheUnavailable = errors.New("redis not configured")
)
