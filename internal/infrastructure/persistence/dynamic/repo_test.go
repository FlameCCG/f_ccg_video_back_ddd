package dynamic_test

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/dynamic"

	domain "f_ccg_video_back_ddd/internal/domain/dynamic"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/dynamic"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	testAPng = "a.png"
)

// fixture 一套用 sqlite 临时库装配的真实仓储与查询（测试不走 wire）。
type fixture struct {
	db        *gorm.DB
	dynamics  *infra.Repository
	likes     *infra.LikeRepository
	reads     *infra.ReadRepository
	activity  *infra.ActivityQuery
	feed      *infra.FeedQuery
	follows   *infra.FollowRelationQuery
	users     *infra.UserProfileQuery
	videos    *infra.VideoOwnershipQuery
	adminList *infra.AdminDynamicQuery
}

// 固定的种子数据：
//   - 用户 1 alice（作者）、2 bob（浏览者，关注了 alice）、3 carol（无关系）
//   - alice 有 1 条已发布视频（id 1）与 2 条动态（id 1、2）
func newFixture(t *testing.T) *fixture {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "dynamic-test.db") + "?_pragma=synchronous(OFF)&_pragma=journal_mode(MEMORY)"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		TranslateError:         true,
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err, "open sqlite failed: %v", err)

	{
		err := db.AutoMigrate(
			&po.User{}, &po.UserFocus{}, &po.Dynamic{}, &po.DynamicLike{},
			&po.DynamicRead{}, &po.Video{}, &po.VideoLike{}, &po.Comment{},
		)
		require.NoError(t, err, "migrate failed: %v", err)
	}

	// 跳过 AfterCreate（会写 user_confs / 默认收藏夹），本测无需这些副作用。
	session := db.Session(&gorm.Session{SkipHooks: true})

	users := []po.User{
		{Model: po.Model{ID: 1}, Username: testAuthorAlice, Email: "a@example.com", Password: "x", Avatar: testAPng},
		{Model: po.Model{ID: 2}, Username: "bob", Email: "b@example.com", Password: "x", Avatar: "b.png"},
		{Model: po.Model{ID: 3}, Username: "carol", Email: "c@example.com", Password: "x", Avatar: "c.png"},
	}
	for i := range users {
		{
			err := session.Create(&users[i]).Error
			require.NoError(t, err, "create user failed: %v", err)
		}
	}

	{
		err := session.Create(&po.UserFocus{UserID: 2, FocusUserID: 1}).Error
		require.NoError(t, err, "create focus failed: %v", err)
	}

	base := time.Now().Add(-24 * time.Hour).Truncate(time.Second)

	video := po.Video{
		Model:          po.Model{ID: 1, CreatedAt: base, UpdatedAt: base},
		Title:          "v1",
		Description:    "d1",
		Cover:          "c1",
		Duration:       60,
		AuthorUsername: testAuthorAlice,
		PartitionID:    1,
		Views:          10,
		Likes:          3,
		CommentCount:   2,
		DanmuCount:     1,
		Status:         enum.VideoStatusPublished,
	}
	{
		err := session.Create(&video).Error
		require.NoError(t, err, "create video failed: %v", err)
	}

	dynamics := []po.Dynamic{
		{Model: po.Model{ID: 1, CreatedAt: base.Add(time.Hour), UpdatedAt: base}, UserID: 1, Content: "first", LikeCount: 1},
		{Model: po.Model{ID: 2, CreatedAt: base.Add(2 * time.Hour), UpdatedAt: base}, UserID: 1, Content: "second"},
	}
	for i := range dynamics {
		{
			err := session.Create(&dynamics[i]).Error
			require.NoError(t, err, "create dynamic failed: %v", err)
		}
	}

	{
		err := session.Create(&po.Comment{
			Content: "nice", UserID: 2, DynamicID: 1, CommentType: enum.DynamicCommentType,
		}).Error
		require.NoError(t, err, "create comment failed: %v", err)
	}

	{
		err := session.Create(&po.DynamicLike{DynamicID: 1, UserID: 2}).Error
		require.NoError(t, err, "create dynamic like failed: %v", err)
	}

	{
		err := session.Create(&po.VideoLike{VideoID: 1, UserID: 2}).Error
		require.NoError(t, err, "create video like failed: %v", err)
	}

	sqlDB, err := db.DB()
	require.NoError(t, err, "db.DB: %v", err)

	t.Cleanup(func() { _ = sqlDB.Close() })

	return &fixture{
		db:        db,
		dynamics:  infra.NewDynamicRepository(db),
		likes:     infra.NewLikeRepository(db),
		reads:     infra.NewReadRepository(db),
		activity:  infra.NewActivityQuery(db),
		feed:      infra.NewFeedQuery(db),
		follows:   infra.NewFollowRelationQuery(db),
		users:     infra.NewUserProfileQuery(db),
		videos:    infra.NewVideoOwnershipQuery(db),
		adminList: infra.NewAdminDynamicQuery(db),
	}
}

func TestDynamicRepositoryCRUD(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()

	created, err := domain.NewDynamic(1, "  hello  ", " img.png ")
	require.NoError(t, err, "NewDynamic: %v", err)

	{
		err := f.dynamics.Create(ctx, created)
		require.NoError(t, err, "Create: %v", err)
	}

	require.NotEqualValues(t, 0, created.ID, "Create 应回填 ID 与 CreatedAt，got %+v", created)
	require.False(t, created.CreatedAt.IsZero(), "Create 应回填 ID 与 CreatedAt，got %+v", created)

	got, err := f.dynamics.FindByID(ctx, created.ID)
	require.NoError(t, err, "FindByID: %v", err)

	require.NotNil(t, got, "FindByID = %+v", got)
	require.Equal(t, "hello", got.Content, "FindByID = %+v", got)
	require.Equal(t, "img.png", got.ImageURL, "FindByID = %+v", got)
	require.EqualValues(t, 1, got.UserID, "FindByID = %+v", got)

	missing, err := f.dynamics.FindByID(ctx, 9999)
	require.NoError(t, err, "FindByID(9999) = %+v, %v; want nil, nil", missing, err)
	require.Nil(t, missing, "FindByID(9999) = %+v, %v; want nil, nil", missing, err)

	exists, err := f.dynamics.ExistsByID(ctx, created.ID)
	require.NoError(t, err, "ExistsByID = %v, %v; want true, nil", exists, err)
	require.True(t, exists, "ExistsByID = %v, %v; want true, nil", exists, err)

	exists, err = f.dynamics.ExistsByID(ctx, 9999)
	require.NoError(t, err, "ExistsByID(9999) = %v, %v; want false, nil", exists, err)
	require.False(t, exists, "ExistsByID(9999) = %v, %v; want false, nil", exists, err)

	notice, err := f.dynamics.FindNoticeTargetByID(ctx, created.ID)
	require.NoError(t, err, "FindNoticeTargetByID = %+v, %v", notice, err)
	require.NotNil(t, notice, "FindNoticeTargetByID = %+v, %v", notice, err)
	require.Equal(t, "hello", notice.Content, "FindNoticeTargetByID = %+v, %v", notice, err)
	require.EqualValues(t, 1, notice.UserID, "FindNoticeTargetByID = %+v, %v", notice, err)

	{
		err := f.dynamics.DeleteByID(ctx, created.ID)
		require.NoError(t, err, "DeleteByID: %v", err)
	}

	{
		got, err := f.dynamics.FindByID(ctx, created.ID)
		require.NoError(t, err, "删除后 FindByID = %+v, %v; want nil, nil", got, err)
		require.Nil(t, got, "删除后 FindByID = %+v, %v; want nil, nil", got, err)
	}

	affected, err := f.dynamics.DeleteByIDs(ctx, []uint{1, 2})
	require.NoError(t, err, "DeleteByIDs: %v", err)

	assert.EqualValues(t, 2, affected, "DeleteByIDs affected = %d, want 2", affected)
}

func TestDynamicRepositoryPinAndLikeCount(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()

	now := time.Now().Truncate(time.Second)
	{
		err := f.dynamics.SetPinned(ctx, 1, &now)
		require.NoError(t, err, "SetPinned: %v", err)
	}

	pinned, err := f.dynamics.FindByID(ctx, 1)
	require.NoError(t, err, "FindByID: %v", err)

	require.True(t, pinned.IsPinned, "SetPinned 后 = %+v", pinned)
	require.NotNil(t, pinned.PinnedAt, "SetPinned 后 = %+v", pinned)

	{
		err := f.dynamics.ClearPinnedByUser(ctx, 1)
		require.NoError(t, err, "ClearPinnedByUser: %v", err)
	}

	cleared, err := f.dynamics.FindByID(ctx, 1)
	require.NoError(t, err, "FindByID: %v", err)

	require.False(t, cleared.IsPinned, "ClearPinnedByUser 后 = %+v", cleared)
	require.Nil(t, cleared.PinnedAt, "ClearPinnedByUser 后 = %+v", cleared)

	{
		err := f.dynamics.SetPinned(ctx, 2, &now)
		require.NoError(t, err, "SetPinned(2): %v", err)
	}

	{
		err := f.dynamics.ClearPinned(ctx, 2)
		require.NoError(t, err, "ClearPinned: %v", err)
	}

	{
		got, _ := f.dynamics.FindByID(ctx, 2)
		assert.False(t, got.IsPinned, "ClearPinned 未生效")
	}

	// like_count 递增与读取（递减用到 MySQL 的 GREATEST，sqlite 不支持，故此处不覆盖）。
	{
		err := f.dynamics.IncrLikeCount(ctx, 1)
		require.NoError(t, err, "IncrLikeCount: %v", err)
	}

	count, err := f.dynamics.LikeCount(ctx, 1)
	require.NoError(t, err, "LikeCount: %v", err)

	assert.EqualValues(t, 2, count, "LikeCount = %d, want 2", count)
}

func TestLikeRepository(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()

	{
		err := f.likes.Create(ctx, 2, 3)
		require.NoError(t, err, "Create: %v", err)
	}

	affected, err := f.likes.Delete(ctx, 2, 3)
	require.NoError(t, err, "Delete: %v", err)

	assert.EqualValues(t, 1, affected, "Delete affected = %d, want 1", affected)

	affected, err = f.likes.Delete(ctx, 2, 3)
	require.NoError(t, err, "Delete again: %v", err)

	assert.EqualValues(t, 0, affected, "重复删除 affected = %d, want 0", affected)
}

func TestReadRepository(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()

	missing, err := f.reads.FindByUserAndAuthor(ctx, 2, 1)
	require.NoError(t, err, "未写入时 FindByUserAndAuthor = %+v, %v; want nil, nil", missing, err)
	require.Nil(t, missing, "未写入时 FindByUserAndAuthor = %+v, %v; want nil, nil", missing, err)

	at := time.Now().Add(-time.Hour).Truncate(time.Second)
	record := &domain.ReadRecord{
		UserID:           2,
		AuthorID:         1,
		LastReadWorkType: enum.WorkTypeDynamic,
		LastReadWorkID:   1,
		LastReadWorkAt:   at,
	}

	{
		err := f.reads.Create(ctx, record)
		require.NoError(t, err, "Create: %v", err)
	}

	assert.NotEqualValues(t, 0, record.ID, "Create 应回填自增 ID")

	got, err := f.reads.FindByUserAndAuthor(ctx, 2, 1)
	require.NoError(t, err, "FindByUserAndAuthor = %+v, %v", got, err)
	require.NotNil(t, got, "FindByUserAndAuthor = %+v, %v", got, err)

	require.EqualValues(t, 1, got.LastReadWorkID, "读到的位点 = %+v", got)
	require.Equal(t, enum.WorkTypeDynamic, got.LastReadWorkType, "读到的位点 = %+v", got)

	next := at.Add(time.Minute)
	{
		err := f.reads.Advance(ctx, got.ID, enum.WorkTypeVideo, 1, next)
		require.NoError(t, err, "Advance: %v", err)
	}

	advanced, err := f.reads.FindByUserAndAuthor(ctx, 2, 1)
	require.NoError(t, err, "FindByUserAndAuthor: %v", err)

	require.Equal(t, enum.WorkTypeVideo, advanced.LastReadWorkType, "Advance 后 = %+v, want video @ %v", advanced, next)
	require.True(t, advanced.LastReadWorkAt.Equal(next), "Advance 后 = %+v, want video @ %v", advanced, next)
}

func TestUserProfileAndFollowQueries(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()

	profile, err := f.users.ProfileByID(ctx, 1)
	require.NoError(t, err, "ProfileByID = %+v, %v", profile, err)
	require.NotNil(t, profile, "ProfileByID = %+v, %v", profile, err)

	require.Equal(t, testAuthorAlice, profile.Username, "profile = %+v", profile)
	require.Equal(t, testAPng, profile.Avatar, "profile = %+v", profile)

	{
		got, err := f.users.ProfileByID(ctx, 9999)
		require.NoError(t, err, "ProfileByID(9999) = %+v, %v; want nil, nil", got, err)
		require.Nil(t, got, "ProfileByID(9999) = %+v, %v; want nil, nil", got, err)
	}

	username, err := f.users.UsernameByID(ctx, 2)
	require.NoError(t, err, "UsernameByID = %q, %v", username, err)
	require.Equal(t, "bob", username, "UsernameByID = %q, %v", username, err)

	{
		username, err := f.users.UsernameByID(ctx, 9999)
		require.NoError(t, err, "UsernameByID(9999) = %q, %v; want \"\", nil", username, err)
		require.Empty(t, username, "UsernameByID(9999) = %q, %v; want \"\", nil", username, err)
	}

	assert.True(t, f.follows.IsFollowing(ctx, 2, 1), "bob 应已关注 alice")

	assert.False(t, f.follows.IsFollowing(ctx, 3, 1), "carol 未关注 alice")

	relations, err := f.follows.ListFollowing(ctx, 2)
	require.NoError(t, err, "ListFollowing: %v", err)

	require.Len(t, relations, 1, "ListFollowing = %+v", relations)
	require.EqualValues(t, 1, relations[0].FocusUserID, "ListFollowing = %+v", relations)
	require.Equal(t, testAuthorAlice, relations[0].Username, "ListFollowing = %+v", relations)

	assert.False(t, relations[0].FollowedAt.IsZero(), "FollowedAt 应取自 user_focus.created_at")

	owned, err := f.videos.ExistsPublishedByAuthor(ctx, 1, testAuthorAlice)
	require.NoError(t, err, "ExistsPublishedByAuthor(1, alice) = %v, %v; want true", owned, err)
	require.True(t, owned, "ExistsPublishedByAuthor(1, alice) = %v, %v; want true", owned, err)

	owned, err = f.videos.ExistsPublishedByAuthor(ctx, 1, "bob")
	require.NoError(t, err, "ExistsPublishedByAuthor(1, bob) = %v, %v; want false", owned, err)
	require.False(t, owned, "ExistsPublishedByAuthor(1, bob) = %v, %v; want false", owned, err)
}

func TestActivityQuery(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()

	// LatestVideoTimes / LatestDynamicTimes 用 MAX(created_at) 聚合：
	// MySQL（DSN 带 parseTime=True）返回 DATETIME，可直接扫进 time.Time；
	// sqlite 的聚合结果会丢失列类型亲和性、返回字符串，故这两个方法不在离线测试内覆盖。
	// 空入参的短路分支仍可断言。
	{
		got, err := f.activity.LatestVideoTimes(ctx, nil)
		require.NoError(t, err, "LatestVideoTimes(nil) = %+v, %v; want empty", got, err)
		require.Empty(t, got, "LatestVideoTimes(nil) = %+v, %v; want empty", got, err)
	}

	{
		got, err := f.activity.LatestDynamicTimes(ctx, nil)
		require.NoError(t, err, "LatestDynamicTimes(nil) = %+v, %v; want empty", got, err)
		require.Empty(t, got, "LatestDynamicTimes(nil) = %+v, %v; want empty", got, err)
	}

	videoWork, err := f.activity.LatestVideoWork(ctx, testAuthorAlice)
	require.NoError(t, err, "LatestVideoWork = %+v, %v", videoWork, err)
	require.EqualValues(t, 1, videoWork.ID, "LatestVideoWork = %+v, %v", videoWork, err)

	{
		got, err := f.activity.LatestVideoWork(ctx, "carol")
		require.NoError(t, err, "LatestVideoWork(carol) = %+v, %v; want zero", got, err)
		require.EqualValues(t, 0, got.ID, "LatestVideoWork(carol) = %+v, %v; want zero", got, err)
	}

	dynamicWork, err := f.activity.LatestDynamicWork(ctx, 1)
	require.NoError(t, err, "LatestDynamicWork = %+v, %v; want 最新的动态 2", dynamicWork, err)
	require.EqualValues(t, 2, dynamicWork.ID, "LatestDynamicWork = %+v, %v; want 最新的动态 2", dynamicWork, err)

	ref, err := f.activity.PublishedVideoAuthor(ctx, 1)
	require.NoError(t, err, "PublishedVideoAuthor: %v", err)

	require.EqualValues(t, 1, ref.VideoID, "PublishedVideoAuthor = %+v", ref)
	require.EqualValues(t, 1, ref.AuthorID, "PublishedVideoAuthor = %+v", ref)
	require.Equal(t, testAuthorAlice, ref.AuthorUsername, "PublishedVideoAuthor = %+v", ref)

	{
		got, err := f.activity.PublishedVideoAuthor(ctx, 9999)
		require.NoError(t, err, "PublishedVideoAuthor(9999) = %+v, %v; want zero", got, err)
		require.EqualValues(t, 0, got.VideoID, "PublishedVideoAuthor(9999) = %+v, %v; want zero", got, err)
	}

	readTimes, err := f.activity.LastReadTimes(ctx, 2, []uint{1})
	require.NoError(t, err, "LastReadTimes: %v", err)

	require.Empty(t, readTimes, "未写入已读位点时应为空，got %+v", readTimes)
}

func TestFeedQuery(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()

	sources := domain.BuildFeedSources(0)

	rows, err := f.feed.ListAuthorWorks(ctx, app.AuthorFeedParams{
		AuthorID:       1,
		AuthorUsername: testAuthorAlice,
		AuthorAvatar:   testAPng,
		Sources:        sources,
		Offset:         0,
		Limit:          10,
	})
	require.NoError(t, err, "ListAuthorWorks: %v", err)

	require.Len(t, rows, 3, "应返回 1 个视频 + 2 条动态，got %d 行: %+v", len(rows), rows)

	// 排序：无置顶时按 created_at 倒序 → 动态 2、动态 1、视频 1。
	assert.False(t, enum.WorkType(rows[0].WorkType) != enum.WorkTypeDynamic || rows[0].WorkID != 2, "首行 = type %d id %d, want 动态 2", rows[0].WorkType, rows[0].WorkID)

	assert.False(t, enum.WorkType(rows[2].WorkType) != enum.WorkTypeVideo || rows[2].WorkID != 1, "末行 = type %d id %d, want 视频 1", rows[2].WorkType, rows[2].WorkID)

	for _, r := range rows {
		require.EqualValues(t, 1, r.AuthorID, "作者列未按占位参数回填: %+v", r)
		require.Equal(t, testAuthorAlice, r.AuthorUsername, "作者列未按占位参数回填: %+v", r)
		require.Equal(t, testAPng, r.AuthorAvatar, "作者列未按占位参数回填: %+v", r)
	}

	videoRow := rows[2]
	assert.False(t, videoRow.VideoTitle != "v1" || videoRow.VideoViews != 10 || videoRow.VideoLikeCount != 3 ||
		videoRow.VideoComment != 2 || videoRow.VideoDanmu != 1 || videoRow.VideoDuration != 60, "视频行统计列 = %+v", videoRow)

	assert.False(t, videoRow.DynContent != "" || videoRow.DynImageURL != "", "视频行的动态列应为 NULL→零值, got %+v", videoRow)

	assert.False(t, rows[1].DynContent != "first" || rows[1].DynLikeCount != 1, "动态行 = %+v", rows[1])

	// 仅视频 / 仅动态。
	rows, err = f.feed.ListAuthorWorks(ctx, app.AuthorFeedParams{
		AuthorID: 1, AuthorUsername: testAuthorAlice, Sources: domain.BuildFeedSources(1), Limit: 10,
	})
	require.False(t, err != nil || len(rows) != 1, "仅视频 = %d 行, %v", len(rows), err)

	rows, err = f.feed.ListAuthorWorks(ctx, app.AuthorFeedParams{
		AuthorID: 1, AuthorUsername: testAuthorAlice, Sources: domain.BuildFeedSources(2), Limit: 10,
	})
	require.False(t, err != nil || len(rows) != 2, "仅动态 = %d 行, %v", len(rows), err)

	total, err := f.feed.CountAuthorWorks(ctx, app.AuthorFeedCountParams{
		AuthorID: 1, AuthorUsername: testAuthorAlice, Sources: sources,
	})
	require.NoError(t, err, "CountAuthorWorks = %d, %v; want 3", total, err)
	require.EqualValues(t, 3, total, "CountAuthorWorks = %d, %v; want 3", total, err)

	comments, err := f.feed.CommentCountsByDynamic(ctx, []uint{1, 2})
	require.NoError(t, err, "CommentCountsByDynamic: %v", err)

	require.EqualValues(t, 1, comments[1], "CommentCountsByDynamic = %+v", comments)
	require.EqualValues(t, 0, comments[2], "CommentCountsByDynamic = %+v", comments)

	likedDynamics, err := f.feed.LikedDynamicIDs(ctx, 2, []uint{1, 2})
	require.NoError(t, err, "LikedDynamicIDs: %v", err)

	require.True(t, likedDynamics[1], "LikedDynamicIDs = %+v", likedDynamics)
	require.False(t, likedDynamics[2], "LikedDynamicIDs = %+v", likedDynamics)

	likedVideos, err := f.feed.LikedVideoIDs(ctx, 2, []uint{1})
	require.NoError(t, err, "LikedVideoIDs: %v", err)

	require.True(t, likedVideos[1], "LikedVideoIDs = %+v", likedVideos)
}

// TestFeedQueryOwnerViewIncludesUnpublished 作者本人视角可见未发布视频。
func TestFeedQueryOwnerViewIncludesUnpublished(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()

	{
		err := f.db.Session(&gorm.Session{SkipHooks: true}).Create(&po.Video{
			Model: po.Model{ID: 2}, Title: "draft", AuthorUsername: testAuthorAlice,
			PartitionID: 1, Status: enum.VideoStatusReview,
		}).Error
		require.NoError(t, err, "create draft video: %v", err)
	}

	params := app.AuthorFeedParams{
		AuthorID: 1, AuthorUsername: testAuthorAlice, Sources: domain.BuildFeedSources(1), Limit: 10,
	}

	rows, err := f.feed.ListAuthorWorks(ctx, params)
	require.False(t, err != nil || len(rows) != 1, "他人视角 = %d 行, %v; want 1", len(rows), err)

	params.IsOwnerView = true

	rows, err = f.feed.ListAuthorWorks(ctx, params)
	require.False(t, err != nil || len(rows) != 2, "本人视角 = %d 行, %v; want 2", len(rows), err)
}

func TestFeedServiceFollowingAggregate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()
	now := time.Now().Add(time.Hour).Truncate(time.Second)
	session := f.db.Session(&gorm.Session{SkipHooks: true})

	for _, video := range []po.Video{
		{
			Model: po.Model{ID: 2, CreatedAt: now}, Title: "self review", AuthorUsername: "bob",
			PartitionID: 1, Status: enum.VideoStatusReview,
		},
		{
			Model: po.Model{ID: 3, CreatedAt: now.Add(time.Minute)}, Title: "followed review", AuthorUsername: testAuthorAlice,
			PartitionID: 1, Status: enum.VideoStatusReview,
		},
	} {
		{
			err := session.Create(&video).Error
			require.NoError(t, err, "create aggregate video: %v", err)
		}
	}

	for _, item := range []po.Dynamic{
		{Model: po.Model{ID: 3, CreatedAt: now.Add(2 * time.Minute)}, UserID: 2, Content: "self dynamic"},
		{Model: po.Model{ID: 4, CreatedAt: now.Add(3 * time.Minute)}, UserID: 3, Content: "stranger dynamic"},
	} {
		{
			err := session.Create(&item).Error
			require.NoError(t, err, "create aggregate dynamic: %v", err)
		}
	}

	service := app.NewFeedService(f.users, f.feed, zap.NewNop())

	firstPage, total, err := service.ListFeed(ctx, 2, 0, 0, 1, 2)
	require.NoError(t, err, "ListFeed first following page: %v", err)

	require.False(t, total != 5 || len(firstPage) != 2, "first following page = %d items total=%d, want 2 and 5", len(firstPage), total)

	items, total, err := service.ListFeed(ctx, 2, 0, 0, 1, 20)
	require.NoError(t, err, "ListFeed following aggregate: %v", err)

	require.False(t, total != 5 || len(items) != 5, "following aggregate = %d items total=%d, want 5", len(items), total)

	var foundSelfReview bool

	for _, item := range items {
		require.NotEqualValues(t, 3, item.Author.ID, "aggregate leaked stranger work: %+v", item)

		require.False(t, item.Video != nil && item.Video.ID == 3, "aggregate leaked followed author's unpublished video: %+v", item)

		if item.Video != nil && item.Video.ID == 2 {
			foundSelfReview = true
		}
	}

	require.True(t, foundSelfReview, "aggregate should include viewer's own unpublished video")

	filters := []struct {
		name     string
		workType int
		want     int
	}{
		{name: "videos", workType: int(enum.WorkTypeVideo), want: 2},
		{name: "dynamics", workType: int(enum.WorkTypeDynamic), want: 3},
	}
	//nolint:paralleltest // Cases share parent fixtures or must finish before their parent resource cleanup.
	for _, filter := range filters {
		t.Run(filter.name, func(t *testing.T) {
			filtered, filteredTotal, filterErr := service.ListFeed(ctx, 2, 0, filter.workType, 1, 20)
			require.NoError(t, filterErr, "ListFeed filtered: %v", filterErr)

			require.False(t, len(filtered) != filter.want || filteredTotal != int64(filter.want), "filtered = %d total=%d, want %d", len(filtered), filteredTotal, filter.want)

			for _, item := range filtered {
				require.Equal(t, int(item.WorkType), filter.workType, "work type = %d, want %d: %+v", item.WorkType, filter.workType, item)
			}
		})
	}

	{
		_, _, err := service.ListFeed(ctx, 0, 0, 0, 1, 20)
		require.ErrorIs(t, err, domain.ErrUserNotLogin, "anonymous following feed error = %v, want %v", err, domain.ErrUserNotLogin)
	}
}

// TestAdminDynamicQueryList 覆盖管理端列表；带 username 过滤会 JOIN users，
// 原实现的裸排序列在此会触发列名歧义（见 report bug 条目），这里断言修复后可用。
func TestAdminDynamicQueryList(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := t.Context()

	items, total, err := f.adminList.List(ctx, app.AdminDynamicListParams{Page: 1, PageSize: 20})
	require.NoError(t, err, "List: %v", err)

	require.False(t, total != 2 || len(items) != 2, "List = %d 项 total=%d", len(items), total)

	require.EqualValues(t, 2, items[0].ID, "首项 = %+v, want 动态 2 + 作者资料", items[0])
	require.Equal(t, testAuthorAlice, items[0].Username, "首项 = %+v, want 动态 2 + 作者资料", items[0])
	require.Equal(t, testAPng, items[0].Avatar, "首项 = %+v, want 动态 2 + 作者资料", items[0])

	items, total, err = f.adminList.List(ctx, app.AdminDynamicListParams{Username: "ali", Page: 1, PageSize: 20})
	require.NoError(t, err, "List with username: %v", err)

	require.EqualValues(t, 2, total, "按用户名过滤 = %+v total=%d", items, total)
	require.Len(t, items, 2, "按用户名过滤 = %+v total=%d", items, total)
	require.EqualValues(t, 2, items[0].ID, "按用户名过滤 = %+v total=%d", items, total)

	items, total, err = f.adminList.List(ctx, app.AdminDynamicListParams{Keyword: "first", Page: 1, PageSize: 20})
	require.NoError(t, err, "List with keyword: %v", err)

	require.EqualValues(t, 1, total, "按关键字过滤 = %+v total=%d", items, total)
	require.Len(t, items, 1, "按关键字过滤 = %+v total=%d", items, total)
	require.EqualValues(t, 1, items[0].ID, "按关键字过滤 = %+v total=%d", items, total)

	items, total, err = f.adminList.List(ctx, app.AdminDynamicListParams{UserID: 3, Page: 1, PageSize: 20})
	require.NoError(t, err, "List with userId: %v", err)

	require.EqualValues(t, 0, total, "无匹配时应返回空列表, got %+v total=%d", items, total)
	require.Empty(t, items, "无匹配时应返回空列表, got %+v total=%d", items, total)

	assert.NotNil(t, items, "空结果应返回非 nil 空切片（响应中渲染为 []）")
}

// 编译期确认各实现满足应用层端口与领域仓储接口。
var (
	_ app.AdminDynamicQuery   = (*infra.AdminDynamicQuery)(nil)
	_ app.FeedQuery           = (*infra.FeedQuery)(nil)
	_ app.ActivityQuery       = (*infra.ActivityQuery)(nil)
	_ app.FollowRelationQuery = (*infra.FollowRelationQuery)(nil)
	_ app.UserProfileQuery    = (*infra.UserProfileQuery)(nil)
	_ app.VideoOwnershipQuery = (*infra.VideoOwnershipQuery)(nil)
)
