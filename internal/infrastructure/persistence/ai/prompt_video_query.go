// Package ai 实现站内作品候选的 SQL 读侧投影。
//
// prompt_video_query.go 从数据库取得提示词需要的候选事实，返回 application/ai 模型。
// 本包不调用 AI，也不拥有这些作品的写入规则；查询可跨表，写入仍归所属上下文。
//
// 仓储遵守消费方端口的不存在、错误和 ID 回填约定，不能只看方法名判断 nil 或错误的含义。
package ai

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"strings"

	app "f_ccg_video_back_ddd/internal/application/ai"
	domain "f_ccg_video_back_ddd/internal/domain/ai"

	"gorm.io/gorm"
)

// PromptVideoQuery 「AI 找视频」候选作品的读侧投影实现（原 ai_service/video_prompt.go）。
//
// 按 D-011，读侧投影允许直读 videos / video_parts / tags 表；
// 作品写侧仍归 video 上下文。db 为 nil 时 Available 返回 false，
// 应用服务据此跳过候选注入，与原实现 global.DB == nil 的降级行为一致。
type PromptVideoQuery struct {
	db *gorm.DB
}

var _ app.PromptVideoQuery = (*PromptVideoQuery)(nil)

// NewPromptVideoQuery 构造候选作品查询。
func NewPromptVideoQuery(db *gorm.DB) *PromptVideoQuery {
	return &PromptVideoQuery{db: db}
}

// Available 报告底层数据库是否可用。
func (q *PromptVideoQuery) Available() bool {
	return q != nil && q.db != nil
}

// KeywordSearchIDs 关键词命中已发布作品 ID。
//
// SQL 与原 searchPromptVideoIDs 的关键词分支逐字一致：标题/描述/分P标题/tag 名模糊匹配，
// 再按「首个关键词的命中位置」优先级、发布时间、ID 倒序排序。
func (q *PromptVideoQuery) KeywordSearchIDs(ctx context.Context, keywords []string, limit int) ([]uint, error) {
	if !q.Available() {
		return nil, nil
	}

	limit = domain.EnsurePositiveLimit(limit)

	query := gormx.DB(ctx, q.db).
		Table("videos").
		Joins("LEFT JOIN video_parts ON video_parts.video_id = videos.id").
		Joins("LEFT JOIN video_tags ON video_tags.video_id = videos.id").
		Joins("LEFT JOIN tags ON tags.id = video_tags.tag_id").
		Where("videos.status = ?", enum.VideoStatusPublished)

	conditions := make([]string, 0, len(keywords))

	args := make([]any, 0, len(keywords)*4)

	for _, keyword := range keywords {
		like := "%" + keyword + "%"

		conditions = append(conditions,
			"(videos.title LIKE ? OR videos.description LIKE ? OR video_parts.title LIKE ? OR tags.name LIKE ?)")
		args = append(args, like, like, like, like)
	}

	if len(conditions) == 0 {
		return nil, nil
	}

	query = query.Where(strings.Join(conditions, " OR "), args...)

	primary := keywords[0]
	prefix := primary + "%"
	query = query.Select(`
videos.id AS id,
MAX(videos.created_at) AS created_at,
MIN(CASE
WHEN videos.title = ? THEN 0
WHEN videos.title LIKE ? THEN 1
WHEN video_parts.title = ? THEN 2
WHEN video_parts.title LIKE ? THEN 3
WHEN tags.name = ? THEN 4
WHEN tags.name LIKE ? THEN 5
WHEN videos.description LIKE ? THEN 6
ELSE 7
END) AS match_rank
`, primary, prefix, primary, prefix, primary, prefix, "%"+primary+"%").
		Group("videos.id").
		Order("match_rank ASC").
		Order("created_at DESC").
		Order("videos.id DESC").
		Limit(limit)

	var rows []struct {
		ID uint `gorm:"column:id"`
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	ids := make([]uint, 0, len(rows))

	for _, row := range rows {
		if row.ID > 0 {
			ids = append(ids, row.ID)
		}
	}

	return ids, nil
}

// LatestPublishedIDs 最新发布作品 ID（关键词未命中时的回退）。
func (q *PromptVideoQuery) LatestPublishedIDs(ctx context.Context, limit int) ([]uint, error) {
	if !q.Available() {
		return nil, nil
	}

	limit = domain.EnsurePositiveLimit(limit)

	var ids []uint

	err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Where("status = ?", enum.VideoStatusPublished).
		Order("created_at DESC").
		Order("id DESC").
		Limit(limit).
		Pluck("id", &ids).Error

	return ids, err
}

// Load 按给定 ID 顺序加载候选作品原始数据（缺失的 ID 直接跳过）。
func (q *PromptVideoQuery) Load(ctx context.Context, ids []uint) ([]domain.VideoCandidateInput, error) {
	if !q.Available() || len(ids) == 0 {
		return nil, nil
	}

	var videos []po.Video
	if err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Select("id", "title", "description", "cover", "partition_id", "status", "created_at").
		Where("id IN ? AND status = ?", ids, enum.VideoStatusPublished).
		Preload("Tags", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name")
		}).
		Preload("VideoPartition", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name")
		}).
		Preload("VideoParts", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "video_id", "title", "sort_order").Order("sort_order ASC, id ASC")
		}).
		Find(&videos).Error; err != nil {
		return nil, err
	}

	byID := make(map[uint]po.Video, len(videos))
	for _, video := range videos {
		byID[video.ID] = video
	}

	out := make([]domain.VideoCandidateInput, 0, len(ids))

	for _, id := range ids {
		video, ok := byID[id]
		if !ok {
			continue
		}

		out = append(out, toCandidateInput(video))
	}

	return out, nil
}

// toCandidateInput 作品 PO → 领域候选构造入参。
func toCandidateInput(video po.Video) domain.VideoCandidateInput {
	input := domain.VideoCandidateInput{
		VideoID:     video.ID,
		Title:       video.Title,
		Description: video.Description,
		Partition:   video.VideoPartition.Name,
		Cover:       video.Cover,
		Tags:        make([]string, 0, len(video.Tags)),
		Parts:       make([]domain.VideoPartInput, 0, len(video.VideoParts)),
	}

	for _, tag := range video.Tags {
		input.Tags = append(input.Tags, tag.Name)
	}

	for _, part := range video.VideoParts {
		input.Parts = append(input.Parts, domain.VideoPartInput{Title: part.Title})
	}

	return input
}
