package video

import "errors"

// 应用层错误（文案与原 service/video_service、api/video_api 逐字一致）。
var (
	// ErrVectorRebuildInProgress 向量索引重建任务正在进行中。
	//
	// 端口 VectorIndexRebuilder.StartRebuild 返回 (false, nil) 时由应用层转成该错误，
	// 避免跨上下文共享哨兵错误实例（原实现是 search_service.ErrRebuildInProgress）。
	ErrVectorRebuildInProgress = errors.New("向量索引重建任务正在进行中")

	// errQueryTagsFailed 查询标签失败.
	errQueryTagsFailed = errors.New("查询标签失败")
	// errPartitionDisabled 分区未启用（改稿路径的文案，与投稿路径不同）.
	errPartitionDisabled = errors.New("分区未启用")
	// errPartitionMissing 分区不存在（改稿路径的文案）.
	errPartitionMissing = errors.New("分区不存在")
	// errStorageUnavailable 存储服务不可用.
	errStorageUnavailable = errors.New("存储服务不可用")
	// errUploadVideoFirst 请先上传视频文件.
	errUploadVideoFirst = errors.New("请先上传视频文件")
	// errUploadAllPartsFirst 请先上传所有分P视频文件.
	errUploadAllPartsFirst = errors.New("请先上传所有分P视频文件")
	// errCheckUploadFailed 检查上传文件失败.
	errCheckUploadFailed = errors.New("检查上传文件失败")
	// errUploadIncomplete 请先完成视频上传（文件可能已被删除，请重新上传）.
	errUploadIncomplete = errors.New("请先完成视频上传（文件可能已被删除，请重新上传）")
	// errRequestParamInvalid 请求参数错误.
	errRequestParamInvalid = errors.New("请求参数错误")
)
