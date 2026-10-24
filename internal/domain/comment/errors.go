package comment

import "errors"

// 评论流程的领域错误（消息与原 service/comment_service 逐字一致，保证接口错误文案不变）。
var (
	ErrLikeDuplicated = errors.New("comment like already exists")
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

	// ErrRecordNotFound 关联记录（视频 / 动态）缺失。
	//
	// 原实现在删除/置顶鉴权时把 gorm.ErrRecordNotFound 原样透传给接口层，
	// 响应文案即 "record not found"；为 1:1 保留该文案，这里声明同文案的领域哨兵，
	// 由 persistence 层把 gorm.ErrRecordNotFound 映射到它。
	ErrRecordNotFound = errors.New("record not found")

	// ErrLikeCacheUnavailable Redis 未配置，点赞缓存不可用.
	ErrLikeCacheUnavailable = errors.New("redis not configured")
)
