package search

import (
	"context"
	"errors"

	app "f_ccg_video_back_ddd/internal/application/search"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// VectorSourceQuery 读取构建视频向量文档所需的库内数据（原 service/search_service/video_vector.go）。
type VectorSourceQuery struct {
	db *gorm.DB
}

var _ app.VideoVectorSourceQuery = (*VectorSourceQuery)(nil)

// NewVectorSourceQuery 构造向量索引源数据查询（db 允许为 nil）。
func NewVectorSourceQuery(db *gorm.DB) *VectorSourceQuery {
	return &VectorSourceQuery{db: db}
}

// Available 报告数据库句柄是否就绪。
func (q *VectorSourceQuery) Available() bool {
	return q != nil && q.db != nil
}

// Load 读取视频及其标签/分区/分P；不存在时返回 app.ErrVectorSourceNotFound。
func (q *VectorSourceQuery) Load(ctx context.Context, videoID uint) (app.VideoVectorSource, error) {
	if !q.Available() {
		return app.VideoVectorSource{}, app.ErrVectorSourceNotFound
	}

	var video po.Video

	err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Select("id", "title", "description", "cover", "partition_id", "status", "created_at").
		Where("id = ?", videoID).
		Preload("Tags", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name")
		}).
		Preload("VideoPartition", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name")
		}).
		Preload("VideoParts", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "video_id", "title", "sort_order").Order("sort_order ASC, id ASC")
		}).
		Take(&video).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return app.VideoVectorSource{}, app.ErrVectorSourceNotFound
		}

		return app.VideoVectorSource{}, err
	}

	return app.VideoVectorSource{
		ID:          video.ID,
		Title:       video.Title,
		Description: video.Description,
		Cover:       video.Cover,
		Partition:   video.VideoPartition.Name,
		Status:      video.Status,
		CreatedAt:   video.CreatedAt,
		Tags:        tagNamesOf(video.Tags),
		PartTitles:  extractPartTitles(video.VideoParts),
	}, nil
}

// tagNamesOf 取标签名（无标签时返回 nil，与向量文本构造侧的判空一致）。
func tagNamesOf(tags []po.Tag) []string {
	if len(tags) == 0 {
		return nil
	}

	names := make([]string, 0, len(tags))

	for _, tag := range tags {
		if tag.Name == "" {
			continue
		}

		names = append(names, tag.Name)
	}

	return names
}
