package site

// Storage 类型.
type Storage struct {
	MaxChunkSize int64  `mapstructure:"maxChunkSize" json:"maxChunkSize"`   // 单位：MB
	ChunkSize    int64  `mapstructure:"chunkSize" json:"chunkSize"`         // 单位：MB
	MaxFileSize  int64  `mapstructure:"maxFileSize" json:"maxFileSize"`     // 单位：MB
	MaxUploadNum int    `mapstructure:"maxUploadNum" json:"maxUploadNum"`   // 最大上传文件数量
	ChunkDir     string `mapstructure:"chunkDir" json:"chunkDir,omitempty"` // 分块上传临时目录
	Local        *Local `mapstructure:"local" json:"local,omitempty"`
	MinIO        *MinIO `mapstructure:"minio" json:"minio,omitempty"`
}

// Local 类型.
type Local struct {
	Enable bool   `mapstructure:"enable" json:"enable"`
	Path   string `mapstructure:"path" json:"path"`
}

// MinIO 类型.
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
