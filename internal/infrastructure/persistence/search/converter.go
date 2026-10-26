// Package search 实现搜索回退、提及和索引源数据的 SQL 查询。
//
// search_fallback.go/suggest_fallback.go 处理 ES 不可用时的候选读取，其余查询为热词、用户和向量提供源事实。
// 本包面向 application/search DTO 做只读投影，可读取多上下文的表；它不负责修改作品或账号。
// 回退查询与 ES 的排序和匹配能力不必完全等价，应按应用用例的实际策略理解结果。
//
// 仓储遵守消费方端口的不存在、错误和 ID 回填约定，不能只看方法名判断 nil 或错误的含义。
package search

import (
	app "f_ccg_video_back_ddd/internal/application/search"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
)

// userFallbackRow 回退查询用的用户命中行（含粉丝数聚合列）。
type userFallbackRow struct {
	ID            uint   `gorm:"column:id"`
	Username      string `gorm:"column:username"`
	Avatar        string `gorm:"column:avatar"`
	Level         int    `gorm:"column:level"`
	FollowerCount int64  `gorm:"column:follower_count"`
}

// toSearchVideoHit 把视频 PO 转成搜索结果项（回退路径无高亮）。
func toSearchVideoHit(video po.Video) app.VideoHit {
	return app.VideoHit{
		ID:             video.ID,
		Title:          video.Title,
		Cover:          video.Cover,
		AuthorUsername: video.AuthorUsername,
		CreatedAt:      video.CreatedAt,
		Views:          video.Views,
		DanmuCount:     video.DanmuCount,
		Duration:       video.Duration,
		Tags:           extractTagNames(video.Tags),
	}
}

// toSearchUserHit 把用户行转成搜索结果项（回退路径无高亮）。
func toSearchUserHit(row userFallbackRow) app.UserHit {
	return app.UserHit{
		ID:            row.ID,
		Username:      row.Username,
		Avatar:        row.Avatar,
		Level:         row.Level,
		FollowerCount: row.FollowerCount,
	}
}

// extractTagNames 取标签名列表；无标签时返回空切片（保证响应里是 [] 而非 null）。
func extractTagNames(tags []po.Tag) []string {
	if len(tags) == 0 {
		return []string{}
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

// extractPartTitles 取分P标题列表（用于向量文本构造）。
func extractPartTitles(parts []po.Part) []string {
	if len(parts) == 0 {
		return nil
	}

	titles := make([]string, 0, len(parts))

	for _, part := range parts {
		if part.Title == "" {
			continue
		}

		titles = append(titles, part.Title)
	}

	return titles
}
