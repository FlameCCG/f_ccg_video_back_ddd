package site

// Storage 组合上传大小/分片限制、每日不同源文件配额、清理保留期与存储驱动配置。
// 大小字段以 MB 配置，保留期以小时配置；上层用例负责换算，驱动方法中的 size 通常是字节数。
type Storage struct {
	MaxChunkSize                    int64  `mapstructure:"maxChunkSize" json:"maxChunkSize"`         // 单分片上限，单位 MB
	ChunkSize                       int64  `mapstructure:"chunkSize" json:"chunkSize"`               // 默认分片体积（MB）；maxChunkSize 为空时用作上限
	MaxFileSize                     int64  `mapstructure:"maxFileSize" json:"maxFileSize"`           // 单文件上限（MB），上传和合并时校验
	MaxUploadNum                    int    `mapstructure:"maxUploadNum" json:"maxUploadNum"`         // 单个作品的分P上限
	DailyUploadLimit                int    `mapstructure:"dailyUploadLimit" json:"dailyUploadLimit"` // 普通用户每日不同源文件上限
	UnmergedChunkRetentionHours     int    `mapstructure:"unmergedChunkRetentionHours" json:"unmergedChunkRetentionHours"`
	UnpublishedSourceRetentionHours int    `mapstructure:"unpublishedSourceRetentionHours" json:"unpublishedSourceRetentionHours"`
	ChunkDir                        string `mapstructure:"chunkDir" json:"chunkDir,omitempty"` // 分块上传临时目录
	Local                           *Local `mapstructure:"local" json:"local,omitempty"`
	MinIO                           *MinIO `mapstructure:"minio" json:"minio,omitempty"`
}

// Local 配置本地驱动开关与文件根目录；落库路径和进程绝对路径需由驱动转换。
type Local struct {
	Enable bool   `mapstructure:"enable" json:"enable"`
	Path   string `mapstructure:"path" json:"path"`
}

// MinIO 描述对象存储连接、bucket、用户路径前缀与匿名可访问前缀。
// 对象 key 不包含服务端磁盘根目录，PublicPrefixes 只表达存储访问策略，不代替稿件业务可见性。
type MinIO struct {
	Enable           bool     `mapstructure:"enable" json:"enable"`
	UserUploadPrefix string   `mapstructure:"userUploadPrefix" json:"userUploadPrefix"` // 用户上传文件目录前缀
	Endpoint         string   `mapstructure:"endpoint" json:"endpoint"`
	AccessKey        string   `mapstructure:"accessKey" json:"accessKey"`
	SecretKey        string   `mapstructure:"secretKey" json:"secretKey"`
	Bucket           string   `mapstructure:"bucket" json:"bucket"`
	UseSSL           bool     `mapstructure:"useSSL" json:"useSSL"`
	PublicPrefixes   []string `mapstructure:"publicPrefixes" json:"publicPrefixes"` // 需要开放匿名访问的前缀
}
