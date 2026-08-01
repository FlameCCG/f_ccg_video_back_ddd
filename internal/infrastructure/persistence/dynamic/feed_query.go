package dynamic

import (
	"context"
	"strings"
	"time"

	app "f_ccg_video_back_ddd/internal/application/dynamic"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// 作品流联合查询的两个分支 SQL（与原 listAuthorFeed 的原生 SQL 逐字一致）。
//
// 两个分支列数、列名、列序完全对齐，才能用 UNION ALL 合并后统一排序；
// 视频分支的 dyn_is_pinned / dyn_pinned_at 复用视频自己的置顶列。
const (
	feedVideoSQL = `
SELECT
  1 AS work_type,
  v.id AS work_id,
  v.created_at AS created_at,
  ? AS author_id,
  ? AS author_username,
  ? AS author_avatar,
  v.title AS video_title,
  v.description AS video_desc,
  v.cover AS video_cover,
  v.duration AS video_duration,
  v.views AS video_views,
  v.danmu_count AS video_danmu_count,
  v.likes AS video_like_count,
  v.comment_count AS video_comment_count,
  NULL AS dyn_content,
  NULL AS dyn_image_url,
  0 AS dyn_like_count,
  v.is_pinned AS dyn_is_pinned,
  v.pinned_at AS dyn_pinned_at
FROM videos v
WHERE v.author_username = ?`

	feedDynamicSQL = `
SELECT
  2 AS work_type,
  d.id AS work_id,
  d.created_at AS created_at,
  ? AS author_id,
  ? AS author_username,
  ? AS author_avatar,
  NULL AS video_title,
  NULL AS video_desc,
  NULL AS video_cover,
  0 AS video_duration,
  0 AS video_views,
  0 AS video_danmu_count,
  0 AS video_like_count,
  0 AS video_comment_count,
  d.content AS dyn_content,
  d.image_url AS dyn_image_url,
  d.like_count AS dyn_like_count,
  d.is_pinned AS dyn_is_pinned,
  d.pinned_at AS dyn_pinned_at
FROM dynamics d
WHERE d.user_id = ?`

	followingFeedVideoSQL = `
SELECT
  1 AS work_type,
  v.id AS work_id,
  v.created_at AS created_at,
  u.id AS author_id,
  u.username AS author_username,
  u.avatar AS author_avatar,
  v.title AS video_title,
  v.description AS video_desc,
  v.cover AS video_cover,
  v.duration AS video_duration,
  v.views AS video_views,
  v.danmu_count AS video_danmu_count,
  v.likes AS video_like_count,
  v.comment_count AS video_comment_count,
  NULL AS dyn_content,
  NULL AS dyn_image_url,
  0 AS dyn_like_count,
  v.is_pinned AS dyn_is_pinned,
  v.pinned_at AS dyn_pinned_at
FROM videos v
JOIN users u ON u.username = v.author_username
WHERE (
    u.id = ?
    OR EXISTS (
      SELECT 1
      FROM user_focus uf
      WHERE uf.user_id = ?
        AND uf.focus_user_id = u.id
    )
  )
  AND (u.id = ? OR v.status = ?)`

	followingFeedDynamicSQL = `
SELECT
  2 AS work_type,
  d.id AS work_id,
  d.created_at AS created_at,
  u.id AS author_id,
  u.username AS author_username,
  u.avatar AS author_avatar,
  NULL AS video_title,
  NULL AS video_desc,
  NULL AS video_cover,
  0 AS video_duration,
  0 AS video_views,
  0 AS video_danmu_count,
  0 AS video_like_count,
  0 AS video_comment_count,
  d.content AS dyn_content,
  d.image_url AS dyn_image_url,
  d.like_count AS dyn_like_count,
  d.is_pinned AS dyn_is_pinned,
  d.pinned_at AS dyn_pinned_at
FROM dynamics d
JOIN users u ON u.id = d.user_id
WHERE (
    u.id = ?
    OR EXISTS (
      SELECT 1
      FROM user_focus uf
      WHERE uf.user_id = ?
        AND uf.focus_user_id = u.id
    )
  )`

	feedOrderSQL = `
ORDER BY dyn_is_pinned DESC, dyn_pinned_at DESC, created_at DESC, work_id DESC
LIMIT ? OFFSET ?`

	followingFeedOrderSQL = `
ORDER BY created_at DESC, work_type DESC, work_id DESC
LIMIT ? OFFSET ?`
)

// feedRow 联合查询的扫描行。
type feedRow struct {
	WorkType       int8       `gorm:"column:work_type"`
	WorkID         uint       `gorm:"column:work_id"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	AuthorID       uint       `gorm:"column:author_id"`
	AuthorUsername string     `gorm:"column:author_username"`
	AuthorAvatar   string     `gorm:"column:author_avatar"`
	VideoTitle     string     `gorm:"column:video_title"`
	VideoDesc      string     `gorm:"column:video_desc"`
	VideoCover     string     `gorm:"column:video_cover"`
	VideoDuration  int        `gorm:"column:video_duration"`
	VideoViews     int64      `gorm:"column:video_views"`
	VideoDanmu     int64      `gorm:"column:video_danmu_count"`
	VideoLikeCount int64      `gorm:"column:video_like_count"`
	VideoComment   int64      `gorm:"column:video_comment_count"`
	DynContent     string     `gorm:"column:dyn_content"`
	DynImageURL    string     `gorm:"column:dyn_image_url"`
	DynLikeCount   int64      `gorm:"column:dyn_like_count"`
	DynIsPinned    bool       `gorm:"column:dyn_is_pinned"`
	DynPinnedAt    *time.Time `gorm:"column:dyn_pinned_at"`
}

// countRow 分组计数的扫描行。
type countRow struct {
	ID    uint  `gorm:"column:id"`
	Count int64 `gorm:"column:count"`
}

// FeedQuery 作品流查询的 GORM 实现。
type FeedQuery struct {
	db *gorm.DB
}

var _ app.FeedQuery = (*FeedQuery)(nil)

// NewFeedQuery 构造作品流查询。
func NewFeedQuery(db *gorm.DB) *FeedQuery {
	return &FeedQuery{db: db}
}

// ListAuthorWorks 以 UNION ALL 联合查询作者的视频与动态。
func (q *FeedQuery) ListAuthorWorks(ctx context.Context, params app.AuthorFeedParams) ([]app.FeedRow, error) {
	sqlParts := make([]string, 0, 2)
	args := make([]any, 0, 16)

	if params.Sources.IncludeVideo {
		videoSQL := feedVideoSQL

		args = append(args, params.AuthorID, params.AuthorUsername, params.AuthorAvatar, params.AuthorUsername)

		if !params.IsOwnerView {
			videoSQL += " AND v.status = ?"

			args = append(args, enum.VideoStatusPublished)
		}

		sqlParts = append(sqlParts, videoSQL)
	}

	if params.Sources.IncludeDynamic {
		sqlParts = append(sqlParts, feedDynamicSQL)
		args = append(args, params.AuthorID, params.AuthorUsername, params.AuthorAvatar, params.AuthorID)
	}

	if len(sqlParts) == 0 {
		return []app.FeedRow{}, nil
	}

	sql := strings.Join(sqlParts, "\nUNION ALL\n") + feedOrderSQL

	args = append(args, params.Limit, params.Offset)

	var rows []feedRow
	if err := gormx.DB(ctx, q.db).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	return toFeedRows(rows), nil
}

// ListFollowingWorks 以 UNION ALL 联合查询浏览者本人及其关注用户的视频与动态。
func (q *FeedQuery) ListFollowingWorks(ctx context.Context, params app.FollowingFeedParams) ([]app.FeedRow, error) {
	sqlParts := make([]string, 0, 2)
	args := make([]any, 0, 8)

	if params.Sources.IncludeVideo {
		sqlParts = append(sqlParts, followingFeedVideoSQL)
		args = append(args, params.ViewerID, params.ViewerID, params.ViewerID, enum.VideoStatusPublished)
	}

	if params.Sources.IncludeDynamic {
		sqlParts = append(sqlParts, followingFeedDynamicSQL)
		args = append(args, params.ViewerID, params.ViewerID)
	}

	if len(sqlParts) == 0 {
		return []app.FeedRow{}, nil
	}

	sql := strings.Join(sqlParts, "\nUNION ALL\n") + followingFeedOrderSQL
	args = append(args, params.Limit, params.Offset)

	var rows []feedRow
	if err := gormx.DB(ctx, q.db).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	return toFeedRows(rows), nil
}

func toFeedRows(rows []feedRow) []app.FeedRow {
	items := make([]app.FeedRow, 0, len(rows))
	for _, r := range rows {
		items = append(items, app.FeedRow{
			WorkType:       r.WorkType,
			WorkID:         r.WorkID,
			CreatedAt:      r.CreatedAt,
			AuthorID:       r.AuthorID,
			AuthorUsername: r.AuthorUsername,
			AuthorAvatar:   r.AuthorAvatar,
			VideoTitle:     r.VideoTitle,
			VideoDesc:      r.VideoDesc,
			VideoCover:     r.VideoCover,
			VideoDuration:  r.VideoDuration,
			VideoViews:     r.VideoViews,
			VideoDanmu:     r.VideoDanmu,
			VideoLikeCount: r.VideoLikeCount,
			VideoComment:   r.VideoComment,
			DynContent:     r.DynContent,
			DynImageURL:    r.DynImageURL,
			DynLikeCount:   r.DynLikeCount,
			DynIsPinned:    r.DynIsPinned,
			DynPinnedAt:    r.DynPinnedAt,
		})
	}

	return items
}

// CountAuthorWorks 统计作者作品总数（视频数 + 动态数）。
func (q *FeedQuery) CountAuthorWorks(ctx context.Context, params app.AuthorFeedCountParams) (int64, error) {
	var videoCount int64

	if params.Sources.IncludeVideo {
		videoQuery := gormx.DB(ctx, q.db).Model(&po.Video{}).
			Where("author_username = ?", params.AuthorUsername)
		if !params.IsOwnerView {
			videoQuery = videoQuery.Where("status = ?", enum.VideoStatusPublished)
		}

		if err := videoQuery.Count(&videoCount).Error; err != nil {
			return 0, err
		}
	}

	var dynamicCount int64
	if params.Sources.IncludeDynamic {
		if err := gormx.DB(ctx, q.db).Model(&po.Dynamic{}).
			Where("user_id = ?", params.AuthorID).
			Count(&dynamicCount).Error; err != nil {
			return 0, err
		}
	}

	return videoCount + dynamicCount, nil
}

// CountFollowingWorks 统计浏览者本人及其关注用户的作品总数。
func (q *FeedQuery) CountFollowingWorks(ctx context.Context, params app.FollowingFeedCountParams) (int64, error) {
	var videoCount int64
	if params.Sources.IncludeVideo {
		if err := gormx.DB(ctx, q.db).
			Table("videos AS v").
			Joins("JOIN users u ON u.username = v.author_username").
			Where(`u.id = ? OR EXISTS (
				SELECT 1
				FROM user_focus uf
				WHERE uf.user_id = ?
				  AND uf.focus_user_id = u.id
			)`, params.ViewerID, params.ViewerID).
			Where("u.id = ? OR v.status = ?", params.ViewerID, enum.VideoStatusPublished).
			Count(&videoCount).Error; err != nil {
			return 0, err
		}
	}

	var dynamicCount int64
	if params.Sources.IncludeDynamic {
		if err := gormx.DB(ctx, q.db).
			Table("dynamics AS d").
			Joins("JOIN users u ON u.id = d.user_id").
			Where(`u.id = ? OR EXISTS (
				SELECT 1
				FROM user_focus uf
				WHERE uf.user_id = ?
				  AND uf.focus_user_id = u.id
			)`, params.ViewerID, params.ViewerID).
			Count(&dynamicCount).Error; err != nil {
			return 0, err
		}
	}

	return videoCount + dynamicCount, nil
}

// CommentCountsByDynamic 按动态批量统计评论数。
func (q *FeedQuery) CommentCountsByDynamic(ctx context.Context, dynamicIDs []uint) (map[uint]int64, error) {
	result := make(map[uint]int64, len(dynamicIDs))
	if len(dynamicIDs) == 0 {
		return result, nil
	}

	var rows []countRow
	if err := gormx.DB(ctx, q.db).
		Model(&po.Comment{}).
		Select("dynamic_id AS id, COUNT(*) AS count").
		Where("dynamic_id IN ?", dynamicIDs).
		Group("dynamic_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		result[row.ID] = row.Count
	}

	return result, nil
}

// LikedVideoIDs 返回浏览者已点赞的视频 ID 集合。
func (q *FeedQuery) LikedVideoIDs(ctx context.Context, viewerID uint, videoIDs []uint) (map[uint]bool, error) {
	result := make(map[uint]bool, len(videoIDs))
	if viewerID == 0 || len(videoIDs) == 0 {
		return result, nil
	}

	var ids []uint
	if err := gormx.DB(ctx, q.db).
		Model(&po.VideoLike{}).
		Where("user_id = ? AND video_id IN ?", viewerID, videoIDs).
		Pluck("video_id", &ids).Error; err != nil {
		return nil, err
	}

	for _, id := range ids {
		result[id] = true
	}

	return result, nil
}

// LikedDynamicIDs 返回浏览者已点赞的动态 ID 集合。
func (q *FeedQuery) LikedDynamicIDs(ctx context.Context, viewerID uint, dynamicIDs []uint) (map[uint]bool, error) {
	result := make(map[uint]bool, len(dynamicIDs))
	if viewerID == 0 || len(dynamicIDs) == 0 {
		return result, nil
	}

	var ids []uint
	if err := gormx.DB(ctx, q.db).
		Model(&po.DynamicLike{}).
		Where("user_id = ? AND dynamic_id IN ?", viewerID, dynamicIDs).
		Pluck("dynamic_id", &ids).Error; err != nil {
		return nil, err
	}

	for _, id := range ids {
		result[id] = true
	}

	return result, nil
}
