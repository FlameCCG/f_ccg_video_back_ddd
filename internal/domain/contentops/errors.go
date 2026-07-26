package contentops

import "errors"

// 内容运营上下文的领域错误。
//
// 错误文案会被 interfaces 层直接回给客户端（原实现即 res.FailWithMsg(err.Error(), c)），
// 属于外部契约，不得改动措辞。
var (
	// ErrBannerInvalidParam 轮播图参数错误（缺少主键、实体为空）。
	ErrBannerInvalidParam = errors.New("参数错误")
	// ErrBannerNoUpdatableField 更新请求未携带任何可更新字段。
	ErrBannerNoUpdatableField = errors.New("没有可更新的字段")
	// ErrBannerPartitionTypeUnsupported 非首页类型的轮播图不支持设置分区 ID。
	ErrBannerPartitionTypeUnsupported = errors.New("只有类型1支持设置分区ID")
	// ErrBannerPartitionNotFound 指定的视频分区不存在。
	ErrBannerPartitionNotFound = errors.New("分区不存在")
	// ErrBannerNotFound 轮播图不存在（含读取失败，与原实现一致地统一归为不存在）。
	ErrBannerNotFound = errors.New("轮播图不存在")
)
