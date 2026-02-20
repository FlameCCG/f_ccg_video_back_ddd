// Package search 提供 ES 查询失败回退、热词采样、@ 提及候选、用户搜索文档与向量索引数据的只读 SQL 投影。
//
// 查询可直读 videos、users、tags 和 user_focus；所有写入仍归各自上下文。
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
func toSearchVideoHit(video po.Video) app.SearchVideoHit {
	return app.SearchVideoHit{
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
func toSearchUserHit(row userFallbackRow) app.SearchUserHit {
	return app.SearchUserHit{
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
func extractPartTitles(parts []po.VideoPart) []string {
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
