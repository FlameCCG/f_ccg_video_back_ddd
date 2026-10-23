// Package video 表达视频、分P、弹幕、收藏夹、互动和发布规则。
//
// 从 video.go 与 revision.go 阅读实体行为，再读 status_resolve.go 理解私密、待审与定时发布的组合。
// repository.go 声明写侧契约；页面详情、列表和管理端组合字段定义在 application/video。
// resource.go/upload.go 描述源文件与资源规则，dash_mpd.go 解析协议数据而不发送网络请求。
// 定时意图、审核状态和转码进度是不同维度，不能用一个“已发布”状态推断媒体产物已就绪。
//
// 本包保持纯 Go 规则与端口，不依赖 HTTP、数据库、缓存、消息框架或配置加载器。
// 领域函数尽量显式接收事实与时间，让规则可以脱离运行环境阅读和验证。
package video

import "errors"

// 领域层错误（文案与原 service/video_service 保持逐字一致，保证接口错误信息不变）。
var (
	// ErrVideoIDRequired videoId 不能为空.
	ErrVideoIDRequired = errors.New("videoId 不能为空")
	// ErrUserNotLogin 用户未登录.
	ErrUserNotLogin = errors.New("用户未登录")
	// ErrLoginRequired 请先登录（弹幕相关接口使用该文案）.
	ErrLoginRequired = errors.New("请先登录")
	// ErrParamInvalid 参数错误.
	ErrParamInvalid = errors.New("参数错误")
	// ErrVideoNotFound 视频不存在.
	ErrVideoNotFound = errors.New("视频不存在")
	// ErrVideoNotPublished 视频不存在或未发布.
	ErrVideoNotPublished = errors.New("视频不存在或未发布")
	// ErrVideoAuthorNotFound 视频作者不存在.
	ErrVideoAuthorNotFound = errors.New("视频作者不存在")
	// ErrUserNotFound 用户不存在.
	ErrUserNotFound = errors.New("用户不存在")

	// ErrCoinLimit 对本稿件的投币次数已用完（幂等上限）.
	ErrCoinLimit = errors.New("coin limit reached")
	// ErrCoinInsufficient 硬币余额不足.
	ErrCoinInsufficient = errors.New("coin insufficient")
	// ErrCoinCountInvalid 投币数量必须大于 0.
	ErrCoinCountInvalid = errors.New("投币数量必须大于0")

	// ErrRecordNotFound 记录不存在。
	//
	// 原实现把 gorm.ErrRecordNotFound 一路透传到 handler 用于区分「不存在」；
	// 新架构由领域层提供哨兵错误，避免 application / interfaces 层依赖 GORM。
	ErrRecordNotFound = errors.New("record not found")

	// ErrDanmuIDRequired danmuId 不能为空.
	ErrDanmuIDRequired = errors.New("danmuId 不能为空")
	// ErrDanmuIDsRequired danmuIds 不能为空.
	ErrDanmuIDsRequired = errors.New("danmuIds 不能为空")
	// ErrDanmuNotFound 弹幕不存在.
	ErrDanmuNotFound = errors.New("弹幕不存在")
	// ErrDanmuContentEmpty 弹幕内容不能为空.
	ErrDanmuContentEmpty = errors.New("弹幕内容不能为空")
	// ErrDanmuTimeOffsetNegative timeOffset 不能为负数.
	ErrDanmuTimeOffsetNegative = errors.New("timeOffset 不能为负数")
	// ErrDanmuPositionInvalid position 仅支持 0/1/2.
	ErrDanmuPositionInvalid = errors.New("position 仅支持0/1/2")
	// ErrDanmuDeleteForbidden 无权限删除.
	ErrDanmuDeleteForbidden = errors.New("无权限删除")
	// ErrDanmuPartIDRequired partId 不能为空.
	ErrDanmuPartIDRequired = errors.New("partId 不能为空")
	// ErrDanmuPartNotFound 分P不存在.
	ErrDanmuPartNotFound = errors.New("分P不存在")

	// ErrFolderNameEmpty 收藏夹名称不能为空.
	ErrFolderNameEmpty = errors.New("收藏夹名称不能为空")
	// ErrDefaultFolderUndeletable 默认收藏夹不可删除.
	ErrDefaultFolderUndeletable = errors.New("默认收藏夹不可删除")
	// ErrFolderNotFoundOrForbidden 收藏夹不存在或无权限修改.
	ErrFolderNotFoundOrForbidden = errors.New("收藏夹不存在或无权限修改")

	// ErrPartitionNotFound 选择的分区不存在.
	ErrPartitionNotFound = errors.New("选择的分区不存在")
	// ErrPartitionInactive 选择的分区未启用.
	ErrPartitionInactive = errors.New("选择的分区未启用")
	// ErrPartitionNotSubmittable 该分区暂不支持投稿.
	ErrPartitionNotSubmittable = errors.New("该分区暂不支持投稿")
	// ErrPartitionHasVideos 分区下存在作品，无法删除.
	ErrPartitionHasVideos = errors.New("分区下存在作品，无法删除")

	// ErrVideoIDsRequired videoIds 不能为空.
	ErrVideoIDsRequired = errors.New("videoIds 不能为空")
	// ErrReportIDsRequired reportIds 不能为空.
	ErrReportIDsRequired = errors.New("reportIds 不能为空")
	// ErrReportReasonEmpty 举报原因不能为空.
	ErrReportReasonEmpty = errors.New("举报原因不能为空")
	// ErrReportNoteTooLong 处理说明过长.
	ErrReportNoteTooLong = errors.New("处理说明过长")
	// ErrReportStatusInvalid status 非法.
	ErrReportStatusInvalid = errors.New("status 非法")
	// ErrReportTypeInvalid type 非法.
	ErrReportTypeInvalid = errors.New("type 非法")

	// ErrTagNameEmpty 标签名称不能为空.
	ErrTagNameEmpty = errors.New("标签名称不能为空")
	// ErrTagNameTooLong 标签名称过长.
	ErrTagNameTooLong = errors.New("标签名称过长")

	// ErrVideoStatusInvalid status 非法.
	ErrVideoStatusInvalid = errors.New("status 非法")
	// ErrVideoDeleteForbidden 无权删除该视频.
	ErrVideoDeleteForbidden = errors.New("无权删除该视频")
	// ErrNotLoggedIn 未登录（作者删除视频时的文案）.
	ErrNotLoggedIn = errors.New("未登录")
	// ErrUserIDRequired 用户ID不能为空.
	ErrUserIDRequired = errors.New("用户ID不能为空")
	// ErrActionUnsupported 不支持的动作.
	ErrActionUnsupported = errors.New("不支持的动作")
	// ErrSchedulerNotRunning 视频调度器未运行.
	ErrSchedulerNotRunning = errors.New("video scheduler not running")

	// ErrDuplicateLike 点赞关系唯一键冲突（并发下已存在）.
	ErrDuplicateLike = errors.New("duplicate video like")
	// ErrDuplicateCoin 投币关系唯一键冲突（并发下已存在）.
	ErrDuplicateCoin = errors.New("duplicate video coin")
)

// 投稿落库失败的业务文案（由仓储在单事务内映射，与原 persistVideo 逐字一致）。
var (
	// ErrCreateVideoFailed 创建视频失败.
	ErrCreateVideoFailed = errors.New("创建视频失败")
	// ErrCreateVideoTagsFailed 创建视频标签关系失败.
	ErrCreateVideoTagsFailed = errors.New("创建视频标签关系失败")
	// ErrCreatePartsFailed 创建分P失败.
	ErrCreatePartsFailed = errors.New("创建分P失败")
	// ErrCreatePartResourcesFailed 创建初始分P资源失败.
	ErrCreatePartResourcesFailed = errors.New("创建初始分P资源失败")
	// ErrCreateVideoResourceFailed 创建初始视频资源失败.
	ErrCreateVideoResourceFailed = errors.New("创建初始视频资源失败")
)

// 上传相关错误（文案与原 upload_service.go 一致）。
var (
	errSourceFileNameEmpty = errors.New("源文件名不能为空")
	errFileNameEmpty       = errors.New("fileName 不能为空")
	errFileNameInvalid     = errors.New("fileName 格式无效")
	errTotalChunksInvalid  = errors.New("totalChunks 必须大于 0")
	errTotalChunksTooLarge = errors.New("totalChunks 超出限制")
)

// 房间弹幕分段查询相关错误（仅领域内部构造，文案与原实现一致）。
var (
	errEndRequired      = errors.New("end 不能为空")
	errEndMustBeGreater = errors.New("end 必须大于 start")
	errDateFormat       = errors.New("date 格式应为 YYYY-MM-DD")
	errMonthFormat      = errors.New("month 格式应为 YYYY-MM")
	errInstantRange     = errors.New("start/end 应为 RFC3339 时间，且间隔不超过 32 天")
	errNotAbsoluteURL   = errors.New("not an absolute url")
)
