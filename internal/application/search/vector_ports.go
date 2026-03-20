package search

import (
	"context"
	"errors"
	"strings"
	"time"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// ErrVectorSourceNotFound 表示向量索引源视频在库中不存在。
var ErrVectorSourceNotFound = errors.New("video vector source not found")

// VideoVectorSource 是构建视频向量文档所需的库内数据.
type VideoVectorSource struct {
	ID          uint
	Title       string
	Description string
	Cover       string
	Partition   string
	Status      enum.VideoStatus
	CreatedAt   time.Time
	Tags        []string
	PartTitles  []string
}

// VideoVectorSourceQuery 向量索引源数据查询端口.
type VideoVectorSourceQuery interface {
	// Available 报告数据库句柄是否就绪。
	Available() bool
	// Load 读取视频及其标签/分区/分P；不存在时返回 ErrVectorSourceNotFound。
	Load(ctx context.Context, videoID uint) (VideoVectorSource, error)
}

// VideoVectorRebuildSource 是向量索引全量回填的主键游标读端口。
type VideoVectorRebuildSource interface {
	// Available 报告数据库句柄是否就绪。
	Available() bool
	// CountPublished 统计当前已发布视频总数。
	CountPublished(ctx context.Context) (int64, error)
	// PublishedAfter 按主键游标读取一页已发布视频及向量化所需的全部关联数据。
	PublishedAfter(ctx context.Context, cursor uint, limit int) ([]VideoVectorSource, error)
}

// VectorConfig 是一次向量检索/索引所使用的运行期配置快照.
type VectorConfig struct {
	// Index 向量索引名。
	Index string
	// TopK 向量召回上限。
	TopK int
	// EmbeddingModel 向量模型名。
	EmbeddingModel string
	// EmbeddingAPIKey 方舟 API Key。
	EmbeddingAPIKey string
	// EmbeddingDimensions 向量维度。
	EmbeddingDimensions int
	// TimeoutSec 向量模型调用超时（秒）；<=0 时适配器取默认值。
	TimeoutSec int
	// ImageMode 控制封面图参与索引向量化：auto / on / off。
	ImageMode string
	// AssetBaseURL 用于把相对封面路径补成方舟可访问的绝对地址。
	AssetBaseURL string
	// LocalStorageRoot 是启用本地存储时的根目录；为空表示不可从磁盘读取封面。
	LocalStorageRoot string
	// RRFRankConstant 是向量与关键词召回做 RRF 融合时的排名常数。
	RRFRankConstant int
	// MinSimilarity 是向量召回最低余弦相似度；<=0 表示不设置阈值。
	MinSimilarity float64
}

// EmbedInput 描述一次多模态向量检索输入。
// 文本与图片可同时提供，基础设施适配器会把二者融合进同一向量空间。
type EmbedInput struct {
	Text     string
	ImageURL string
}

// IsZero 判断多模态输入是否同时缺少文本和图片。
func (in EmbedInput) IsZero() bool {
	return strings.TrimSpace(in.Text) == "" && strings.TrimSpace(in.ImageURL) == ""
}

// VideoVectorIndex 视频向量索引端口（ES8 + eino-ext）.
type VideoVectorIndex interface {
	// Available 报告 ES8 客户端是否就绪。
	Available() bool
	// SearchIDs 以文本、图片或二者组合做混合召回，返回去重后的视频 ID（最多 limit 个）。
	SearchIDs(ctx context.Context, cfg VectorConfig, input EmbedInput, limit int) ([]uint, error)
	// Store 覆盖写入一个视频的向量文档。
	Store(ctx context.Context, cfg VectorConfig, source VideoVectorSource) error
	// Delete 删除一个视频的向量文档；文档不存在视为成功。
	Delete(ctx context.Context, cfg VectorConfig, videoID uint) error
}

// VideoVectorIndexLifecycle 是全量回填所需的索引运维端口。
type VideoVectorIndexLifecycle interface {
	// Available 报告 ES8 客户端是否就绪。
	Available() bool
	// Prepare 确保索引存在；recreate 为 true 时先删除再按当前维度创建。
	Prepare(ctx context.Context, cfg VectorConfig, recreate bool) error
	// Refresh 让本次写入立即可检索。
	Refresh(ctx context.Context, cfg VectorConfig) error
}
