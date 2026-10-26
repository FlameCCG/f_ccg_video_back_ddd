package enum

// VideoStatus 是稿件审核/可见性状态，不表示转码进度或资源是否齐备。
type VideoStatus int

// Video and storage enum values retain their persisted representations.
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

// VideoResolution 是资源的业务档位标签；自适应档指向 MPD，实际像素与码率由具体资源提供。
type VideoResolution string

// Video and storage enum values retain their persisted representations.
const (
	// Resolution360P 是流畅档的持久化标签。
	Resolution360P VideoResolution = "360p 流畅"
	// Resolution720P 是准高清档的持久化标签。
	Resolution720P VideoResolution = "720p 准高清"
	// Resolution1080P 是普通高清档标签，与高码率 1080 档分别保存。
	Resolution1080P VideoResolution = "1080p 高清"
	// Resolution1080PHigh identifies the high bitrate 1080p output tier.
	Resolution1080PHigh VideoResolution = "1080p 高码率"
	Resolution2160P     VideoResolution = "4k 超高清"
	ResolutionAuto      VideoResolution = "自适应" // DASH 自适应流(多码率)，FileURL 指向 .mpd
)

// VideoStorageType 是已落库资源使用的驱动类别；历史枚举值不代表当前 Resolver 都提供对应实现。
type VideoStorageType string

// Video and storage enum values retain their persisted representations.
const (
	// StorageTypeLocal 标记本地磁盘驱动。
	StorageTypeLocal VideoStorageType = "本地"
	// StorageTypeMinio 标记 MinIO 对象存储驱动。
	StorageTypeMinio VideoStorageType = "Minio"
	// StorageTypeQiniu 保留七牛云 Kodo 类型值，实际支持需查看存储 Resolver。
	StorageTypeQiniu VideoStorageType = "七牛云Kodo"
	// StorageTypeTencent identifies Tencent object storage.
	StorageTypeTencent VideoStorageType = "腾讯云COS"
	StorageTypeOss     VideoStorageType = "阿里云OSS"
)
