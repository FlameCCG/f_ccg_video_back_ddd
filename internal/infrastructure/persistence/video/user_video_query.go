package video

import (
	"context"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/gormx"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"gorm.io/gorm"
)

// UserVideoQuery 用户投稿列表读模型查询的 GORM 实现。
type UserVideoQuery struct {
	db *gorm.DB
}

var _ app.UserVideoQuery = (*UserVideoQuery)(nil)

// NewUserVideoQuery 构造用户投稿列表查询。
func NewUserVideoQuery(db *gorm.DB) *UserVideoQuery {
	return &UserVideoQuery{db: db}
}

// ListByAuthor 按作者用户名分页读取投稿（isOwnerView 决定可见状态集合）。
//
// 与原 ListUserVideos 的「多取一条探测下一页」优化一致：
// 先取 pageSize+1 条推断 total，只有确实有下一页时才回源 COUNT。
func (q *UserVideoQuery) ListByAuthor(
	ctx context.Context,
	authorUsername string,
	isOwnerView bool,
	auditStatus, sort, page, pageSize int,
) ([]app.UserVideoRow, int64, error) {
	query := gormx.DB(ctx, q.db).Model(&po.Video{}).
		Where("author_username = ?", authorUsername)
	query = applyUserVideoAuditFilter(query, isOwnerView, auditStatus)

	switch sort {
	case domain.UserVideoSortViews:
		query = query.Order("views DESC")
	case domain.UserVideoSortFavorite:
		query = query.Order("favorite_count DESC")
	default:
		query = query.Order("created_at DESC")
	}

	var rows []po.Video
	if err := query.
		Select("id", "title", "cover", "views", "likes", "favorite_count",
			"created_at", "duration", "danmu_count", "status").
		Offset((page - 1) * pageSize).
		Limit(pageSize + 1).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	if len(rows) == 0 {
		return []app.UserVideoRow{}, 0, nil
	}

	total := int64((page-1)*pageSize + len(rows))
	if len(rows) > pageSize {
		rows = rows[:pageSize]
		total = 0
	}

	if total == 0 {
		if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
			return nil, 0, err
		}
	}

	items := make([]app.UserVideoRow, 0, len(rows))
	for i := range rows {
		items = append(items, app.UserVideoRow{
			ID:            rows[i].ID,
			Title:         rows[i].Title,
			Cover:         rows[i].Cover,
			Views:         rows[i].Views,
			Likes:         rows[i].Likes,
			FavoriteCount: rows[i].FavoriteCount,
			DanmuCount:    rows[i].DanmuCount,
			Duration:      rows[i].Duration,
			CreatedAt:     rows[i].CreatedAt,
			Status:        rows[i].Status,
		})
	}

	return items, total, nil
}

// applyUserVideoAuditFilter 按访问者视角与审核态筛选可见视频（与原实现逐分支一致）。
func applyUserVideoAuditFilter(query *gorm.DB, isOwnerView bool, auditStatus int) *gorm.DB {
	if !isOwnerView {
		return query.Where("status = ?", enum.VideoStatusPublished)
	}

	switch auditStatus {
	case domain.UserVideoAuditPublished:
		return query.Where("status = ?", enum.VideoStatusPublished)
	case domain.UserVideoAuditPrivate:
		return query.Where("status = ?", enum.VideoStatusPrivate)
	case domain.UserVideoAuditReviewing:
		return query.Where("status = ?", enum.VideoStatusReview)
	case domain.UserVideoAuditScheduled:
		return query.Where("status = ?", enum.VideoStatusScheduled)
	default:
		return query.Where("status IN ?", domain.OwnerVisibleStatuses())
	}
}

// RecentActionVideos 读取用户最近点赞/投币的视频。
func (q *UserVideoQuery) RecentActionVideos(
	ctx context.Context,
	userID uint,
	action string,
	limit int,
) ([]app.UserVideoRow, error) {
	query := gormx.DB(ctx, q.db).
		Select("v.id, v.title, v.cover, v.views, v.likes, v.favorite_count, " +
			"v.duration, v.danmu_count, v.created_at, v.author_username")

	if action == domain.ActionLike {
		query = query.Table("video_likes vl").
			Joins("JOIN videos v ON v.id = vl.video_id").
			Where("vl.user_id = ?", userID).
			Order("vl.created_at DESC").
			Limit(limit)
	} else {
		query = query.Table("video_coins vc").
			Joins("JOIN videos v ON v.id = vc.video_id").
			Where("vc.user_id = ?", userID).
			Order("vc.created_at DESC").
			Limit(limit)
	}

	var rows []po.Video
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return []app.UserVideoRow{}, nil
	}

	usernames := make([]string, 0, len(rows))
	for i := range rows {
		if rows[i].AuthorUsername != "" {
			usernames = append(usernames, rows[i].AuthorUsername)
		}
	}

	authorMap := q.usersByUsernames(ctx, usernames)

	items := make([]app.UserVideoRow, 0, len(rows))

	for i := range rows {
		authorID := uint(0)

		authorName := rows[i].AuthorUsername
		if author, ok := authorMap[rows[i].AuthorUsername]; ok {
			authorID = author.ID
			if author.Username != "" {
				authorName = author.Username
			}
		}

		items = append(items, app.UserVideoRow{
			ID:             rows[i].ID,
			Title:          rows[i].Title,
			Cover:          rows[i].Cover,
			Views:          rows[i].Views,
			Likes:          rows[i].Likes,
			FavoriteCount:  rows[i].FavoriteCount,
			DanmuCount:     rows[i].DanmuCount,
			Duration:       rows[i].Duration,
			CreatedAt:      rows[i].CreatedAt,
			Status:         rows[i].Status,
			AuthorUsername: rows[i].AuthorUsername,
			AuthorID:       authorID,
			AuthorName:     authorName,
		})
	}

	return items, nil
}

// usersByUsernames 按用户名批量读取用户（去重；查询失败返回 nil）。
func (q *UserVideoQuery) usersByUsernames(ctx context.Context, usernames []string) map[string]po.User {
	if len(usernames) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(usernames))
	unique := make([]string, 0, len(usernames))

	for _, username := range usernames {
		if username == "" {
			continue
		}

		if _, ok := seen[username]; ok {
			continue
		}

		seen[username] = struct{}{}
		unique = append(unique, username)
	}

	if len(unique) == 0 {
		return nil
	}

	var users []po.User
	if err := gormx.DB(ctx, q.db).
		Select("id", "username").
		Where("username IN ?", unique).
		Find(&users).Error; err != nil {
		return nil
	}

	result := make(map[string]po.User, len(users))

	for _, user := range users {
		if user.Username != "" {
			result[user.Username] = user
		}
	}

	return result
}

// AggregateAuthorStats 汇总作者已发布视频的行内 views/likes（含视频 ID 列表以便叠加 Redis 权威计数）。
func (q *UserVideoQuery) AggregateAuthorStats(
	ctx context.Context,
	authorUsername string,
) ([]app.AuthorVideoStatRow, error) {
	type row struct {
		ID    uint  `gorm:"column:id"`
		Views int64 `gorm:"column:views"`
		Likes int64 `gorm:"column:likes"`
	}

	var rows []row
	if err := gormx.DB(ctx, q.db).
		Model(&po.Video{}).
		Select("id", "views", "likes").
		Where("author_username = ? AND status = ?", authorUsername, enum.VideoStatusPublished).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]app.AuthorVideoStatRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, app.AuthorVideoStatRow{ID: r.ID, Views: r.Views, Likes: r.Likes})
	}

	return out, nil
}
