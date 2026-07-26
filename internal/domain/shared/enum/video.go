package enum

// VideoStatus 类型.
type VideoStatus int

// VideoStatusDeleted 及相关导出常量.
const (
	// VideoStatusPublished 已发布（公开可见）.
	VideoStatusPublished VideoStatus = iota + 1
	// VideoStatusPrivate 私密.
	VideoStatusPrivate
	// VideoStatusDeleted 已删除.
	VideoStatusDeleted
	// VideoStatusReview 待人工审核.
	VideoStatusReview
	// VideoStatusScheduled 已通过（或免审）等待定时发布.
	VideoStatusScheduled
)

// VideoResolution 类型.
type VideoResolution string

// Resolution1080P 及相关导出常量.
const (
	// Resolution360P 常量.
	Resolution360P VideoResolution = "360p 流畅"
	// Resolution720P 常量.
	Resolution720P VideoResolution = "720p 准高清"
	// Resolution1080P 常量.
	Resolution1080P     VideoResolution = "1080p 高清"
	Resolution1080PHigh VideoResolution = "1080p 高码率"
	Resolution2160P     VideoResolution = "4k 超高清"
	ResolutionAuto      VideoResolution = "自适应" // DASH 自适应流(多码率)，FileURL 指向 .mpd
)

// VideoStorageType 类型.
type VideoStorageType string

// StorageTypeQiniu 及相关导出常量.
const (
	// StorageTypeLocal 常量.
	StorageTypeLocal VideoStorageType = "本地"
	// StorageTypeMinio 常量.
	StorageTypeMinio VideoStorageType = "Minio"
	// StorageTypeQiniu 常量.
	StorageTypeQiniu   VideoStorageType = "七牛云Kodo"
	StorageTypeTencent VideoStorageType = "腾讯云COS"
	StorageTypeOss     VideoStorageType = "阿里云OSS"
)
