package dynamic_test

import (
	"path/filepath"
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/dynamic"
	domain "f_ccg_video_back_ddd/internal/domain/dynamic"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/dynamic"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// fixture 一套用 sqlite 临时库装配的真实仓储与查询（测试不走 wire）。
type fixture struct {
	db        *gorm.DB
	dynamics  *infra.DynamicRepository
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
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	if err := db.AutoMigrate(
		&po.User{}, &po.UserFocus{}, &po.Dynamic{}, &po.DynamicLike{},
		&po.DynamicRead{}, &po.Video{}, &po.VideoLike{}, &po.Comment{},
	); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	// 跳过 AfterCreate（会写 user_confs / 默认收藏夹），本测无需这些副作用。
	session := db.Session(&gorm.Session{SkipHooks: true})

	users := []po.User{
		{Model: po.Model{ID: 1}, Username: "alice", Email: "a@example.com", Password: "x", Avatar: "a.png"},
		{Model: po.Model{ID: 2}, Username: "bob", Email: "b@example.com", Password: "x", Avatar: "b.png"},
		{Model: po.Model{ID: 3}, Username: "carol", Email: "c@example.com", Password: "x", Avatar: "c.png"},
	}
	for i := range users {
		if err := session.Create(&users[i]).Error; err != nil {
			t.Fatalf("create user failed: %v", err)
		}
	}

	if err := session.Create(&po.UserFocus{UserID: 2, FocusUserID: 1}).Error; err != nil {
		t.Fatalf("create focus failed: %v", err)
	}

	base := time.Now().Add(-24 * time.Hour).Truncate(time.Second)

	video := po.Video{
		Model:          po.Model{ID: 1, CreatedAt: base, UpdatedAt: base},
		Title:          "v1",
		Description:    "d1",
		Cover:          "c1",
		Duration:       60,
		AuthorUsername: "alice",
		PartitionID:    1,
		Views:          10,
		Likes:          3,
		CommentCount:   2,
		DanmuCount:     1,
		Status:         enum.VideoStatusPublished,
	}
	if err := session.Create(&video).Error; err != nil {
		t.Fatalf("create video failed: %v", err)
	}

	dynamics := []po.Dynamic{
		{Model: po.Model{ID: 1, CreatedAt: base.Add(time.Hour), UpdatedAt: base}, UserID: 1, Content: "first", LikeCount: 1},
		{Model: po.Model{ID: 2, CreatedAt: base.Add(2 * time.Hour), UpdatedAt: base}, UserID: 1, Content: "second"},
	}
	for i := range dynamics {
		if err := session.Create(&dynamics[i]).Error; err != nil {
			t.Fatalf("create dynamic failed: %v", err)
		}
	}

	if err := session.Create(&po.Comment{
		Content: "nice", UserID: 2, DynamicID: 1, CommentType: enum.DynamicCommentType,
	}).Error; err != nil {
		t.Fatalf("create comment failed: %v", err)
	}

	if err := session.Create(&po.DynamicLike{DynamicID: 1, UserID: 2}).Error; err != nil {
		t.Fatalf("create dynamic like failed: %v", err)
	}

	if err := session.Create(&po.VideoLike{VideoID: 1, UserID: 2}).Error; err != nil {
		t.Fatalf("create video like failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}

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
	f := newFixture(t)
	ctx := t.Context()

	created, err := domain.NewDynamic(1, "  hello  ", " img.png ")
	if err != nil {
		t.Fatalf("NewDynamic: %v", err)
	}

	if err := f.dynamics.Create(ctx, created); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if created.ID == 0 || created.CreatedAt.IsZero() {
		t.Fatalf("Create 应回填 ID 与 CreatedAt，got %+v", created)
	}

	got, err := f.dynamics.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if got == nil || got.Content != "hello" || got.ImageURL != "img.png" || got.UserID != 1 {
		t.Fatalf("FindByID = %+v", got)
	}

	missing, err := f.dynamics.FindByID(ctx, 9999)
	if err != nil || missing != nil {
		t.Fatalf("FindByID(9999) = %+v, %v; want nil, nil", missing, err)
	}

	exists, err := f.dynamics.ExistsByID(ctx, created.ID)
	if err != nil || !exists {
		t.Fatalf("ExistsByID = %v, %v; want true, nil", exists, err)
	}

	exists, err = f.dynamics.ExistsByID(ctx, 9999)
	if err != nil || exists {
		t.Fatalf("ExistsByID(9999) = %v, %v; want false, nil", exists, err)
	}

	notice, err := f.dynamics.FindNoticeTargetByID(ctx, created.ID)
	if err != nil || notice == nil || notice.Content != "hello" || notice.UserID != 1 {
		t.Fatalf("FindNoticeTargetByID = %+v, %v", notice, err)
	}

	if err := f.dynamics.DeleteByID(ctx, created.ID); err != nil {
		t.Fatalf("DeleteByID: %v", err)
	}

	if got, err := f.dynamics.FindByID(ctx, created.ID); err != nil || got != nil {
		t.Fatalf("删除后 FindByID = %+v, %v; want nil, nil", got, err)
	}

	affected, err := f.dynamics.DeleteByIDs(ctx, []uint{1, 2})
	if err != nil {
		t.Fatalf("DeleteByIDs: %v", err)
	}

	if affected != 2 {
		t.Errorf("DeleteByIDs affected = %d, want 2", affected)
	}
}

func TestDynamicRepositoryPinAndLikeCount(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()

	now := time.Now().Truncate(time.Second)
	if err := f.dynamics.SetPinned(ctx, 1, &now); err != nil {
		t.Fatalf("SetPinned: %v", err)
	}

	pinned, err := f.dynamics.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if !pinned.IsPinned || pinned.PinnedAt == nil {
		t.Fatalf("SetPinned 后 = %+v", pinned)
	}

	if err := f.dynamics.ClearPinnedByUser(ctx, 1); err != nil {
		t.Fatalf("ClearPinnedByUser: %v", err)
	}

	cleared, err := f.dynamics.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if cleared.IsPinned || cleared.PinnedAt != nil {
		t.Fatalf("ClearPinnedByUser 后 = %+v", cleared)
	}

	if err := f.dynamics.SetPinned(ctx, 2, &now); err != nil {
		t.Fatalf("SetPinned(2): %v", err)
	}

	if err := f.dynamics.ClearPinned(ctx, 2); err != nil {
		t.Fatalf("ClearPinned: %v", err)
	}

	if got, _ := f.dynamics.FindByID(ctx, 2); got.IsPinned {
		t.Error("ClearPinned 未生效")
	}

	// like_count 递增与读取（递减用到 MySQL 的 GREATEST，sqlite 不支持，故此处不覆盖）。
	if err := f.dynamics.IncrLikeCount(ctx, 1); err != nil {
		t.Fatalf("IncrLikeCount: %v", err)
	}

	count, err := f.dynamics.LikeCount(ctx, 1)
	if err != nil {
		t.Fatalf("LikeCount: %v", err)
	}

	if count != 2 {
		t.Errorf("LikeCount = %d, want 2", count)
	}
}

func TestLikeRepository(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()

	if err := f.likes.Create(ctx, 2, 3); err != nil {
		t.Fatalf("Create: %v", err)
	}

	affected, err := f.likes.Delete(ctx, 2, 3)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if affected != 1 {
		t.Errorf("Delete affected = %d, want 1", affected)
	}

	affected, err = f.likes.Delete(ctx, 2, 3)
	if err != nil {
		t.Fatalf("Delete again: %v", err)
	}

	if affected != 0 {
		t.Errorf("重复删除 affected = %d, want 0", affected)
	}
}

func TestReadRepository(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()

	missing, err := f.reads.FindByUserAndAuthor(ctx, 2, 1)
	if err != nil || missing != nil {
		t.Fatalf("未写入时 FindByUserAndAuthor = %+v, %v; want nil, nil", missing, err)
	}

	at := time.Now().Add(-time.Hour).Truncate(time.Second)
	record := &domain.ReadRecord{
		UserID:           2,
		AuthorID:         1,
		LastReadWorkType: enum.WorkTypeDynamic,
		LastReadWorkID:   1,
		LastReadWorkAt:   at,
	}

	if err := f.reads.Create(ctx, record); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if record.ID == 0 {
		t.Error("Create 应回填自增 ID")
	}

	got, err := f.reads.FindByUserAndAuthor(ctx, 2, 1)
	if err != nil || got == nil {
		t.Fatalf("FindByUserAndAuthor = %+v, %v", got, err)
	}

	if got.LastReadWorkID != 1 || got.LastReadWorkType != enum.WorkTypeDynamic {
		t.Fatalf("读到的位点 = %+v", got)
	}

	next := at.Add(time.Minute)
	if err := f.reads.Advance(ctx, got.ID, enum.WorkTypeVideo, 1, next); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	advanced, err := f.reads.FindByUserAndAuthor(ctx, 2, 1)
	if err != nil {
		t.Fatalf("FindByUserAndAuthor: %v", err)
	}

	if advanced.LastReadWorkType != enum.WorkTypeVideo || !advanced.LastReadWorkAt.Equal(next) {
		t.Fatalf("Advance 后 = %+v, want video @ %v", advanced, next)
	}
}

func TestUserProfileAndFollowQueries(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()

	profile, err := f.users.ProfileByID(ctx, 1)
	if err != nil || profile == nil {
		t.Fatalf("ProfileByID = %+v, %v", profile, err)
	}

	if profile.Username != "alice" || profile.Avatar != "a.png" {
		t.Fatalf("profile = %+v", profile)
	}

	if got, err := f.users.ProfileByID(ctx, 9999); err != nil || got != nil {
		t.Fatalf("ProfileByID(9999) = %+v, %v; want nil, nil", got, err)
	}

	username, err := f.users.UsernameByID(ctx, 2)
	if err != nil || username != "bob" {
		t.Fatalf("UsernameByID = %q, %v", username, err)
	}

	if username, err := f.users.UsernameByID(ctx, 9999); err != nil || username != "" {
		t.Fatalf("UsernameByID(9999) = %q, %v; want \"\", nil", username, err)
	}

	if !f.follows.IsFollowing(ctx, 2, 1) {
		t.Error("bob 应已关注 alice")
	}

	if f.follows.IsFollowing(ctx, 3, 1) {
		t.Error("carol 未关注 alice")
	}

	relations, err := f.follows.ListFollowing(ctx, 2)
	if err != nil {
		t.Fatalf("ListFollowing: %v", err)
	}

	if len(relations) != 1 || relations[0].FocusUserID != 1 || relations[0].Username != "alice" {
		t.Fatalf("ListFollowing = %+v", relations)
	}

	if relations[0].FollowedAt.IsZero() {
		t.Error("FollowedAt 应取自 user_focus.created_at")
	}

	owned, err := f.videos.ExistsPublishedByAuthor(ctx, 1, "alice")
	if err != nil || !owned {
		t.Fatalf("ExistsPublishedByAuthor(1, alice) = %v, %v; want true", owned, err)
	}

	owned, err = f.videos.ExistsPublishedByAuthor(ctx, 1, "bob")
	if err != nil || owned {
		t.Fatalf("ExistsPublishedByAuthor(1, bob) = %v, %v; want false", owned, err)
	}
}

func TestActivityQuery(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()

	// LatestVideoTimes / LatestDynamicTimes 用 MAX(created_at) 聚合：
	// MySQL（DSN 带 parseTime=True）返回 DATETIME，可直接扫进 time.Time；
	// sqlite 的聚合结果会丢失列类型亲和性、返回字符串，故这两个方法不在离线测试内覆盖。
	// 空入参的短路分支仍可断言。
	if got, err := f.activity.LatestVideoTimes(ctx, nil); err != nil || len(got) != 0 {
		t.Fatalf("LatestVideoTimes(nil) = %+v, %v; want empty", got, err)
	}

	if got, err := f.activity.LatestDynamicTimes(ctx, nil); err != nil || len(got) != 0 {
		t.Fatalf("LatestDynamicTimes(nil) = %+v, %v; want empty", got, err)
	}

	videoWork, err := f.activity.LatestVideoWork(ctx, "alice")
	if err != nil || videoWork.ID != 1 {
		t.Fatalf("LatestVideoWork = %+v, %v", videoWork, err)
	}

	if got, err := f.activity.LatestVideoWork(ctx, "carol"); err != nil || got.ID != 0 {
		t.Fatalf("LatestVideoWork(carol) = %+v, %v; want zero", got, err)
	}

	dynamicWork, err := f.activity.LatestDynamicWork(ctx, 1)
	if err != nil || dynamicWork.ID != 2 {
		t.Fatalf("LatestDynamicWork = %+v, %v; want 最新的动态 2", dynamicWork, err)
	}

	ref, err := f.activity.PublishedVideoAuthor(ctx, 1)
	if err != nil {
		t.Fatalf("PublishedVideoAuthor: %v", err)
	}

	if ref.VideoID != 1 || ref.AuthorID != 1 || ref.AuthorUsername != "alice" {
		t.Fatalf("PublishedVideoAuthor = %+v", ref)
	}

	if got, err := f.activity.PublishedVideoAuthor(ctx, 9999); err != nil || got.VideoID != 0 {
		t.Fatalf("PublishedVideoAuthor(9999) = %+v, %v; want zero", got, err)
	}

	readTimes, err := f.activity.LastReadTimes(ctx, 2, []uint{1})
	if err != nil {
		t.Fatalf("LastReadTimes: %v", err)
	}

	if len(readTimes) != 0 {
		t.Fatalf("未写入已读位点时应为空，got %+v", readTimes)
	}
}

func TestFeedQuery(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()

	sources := domain.BuildFeedSources(0)

	rows, err := f.feed.ListAuthorWorks(ctx, app.AuthorFeedParams{
		AuthorID:       1,
		AuthorUsername: "alice",
		AuthorAvatar:   "a.png",
		Sources:        sources,
		Offset:         0,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("ListAuthorWorks: %v", err)
	}

	if len(rows) != 3 {
		t.Fatalf("应返回 1 个视频 + 2 条动态，got %d 行: %+v", len(rows), rows)
	}

	// 排序：无置顶时按 created_at 倒序 → 动态 2、动态 1、视频 1。
	if enum.WorkType(rows[0].WorkType) != enum.WorkTypeDynamic || rows[0].WorkID != 2 {
		t.Errorf("首行 = type %d id %d, want 动态 2", rows[0].WorkType, rows[0].WorkID)
	}

	if enum.WorkType(rows[2].WorkType) != enum.WorkTypeVideo || rows[2].WorkID != 1 {
		t.Errorf("末行 = type %d id %d, want 视频 1", rows[2].WorkType, rows[2].WorkID)
	}

	for _, r := range rows {
		if r.AuthorID != 1 || r.AuthorUsername != "alice" || r.AuthorAvatar != "a.png" {
			t.Fatalf("作者列未按占位参数回填: %+v", r)
		}
	}

	videoRow := rows[2]
	if videoRow.VideoTitle != "v1" || videoRow.VideoViews != 10 || videoRow.VideoLikeCount != 3 ||
		videoRow.VideoComment != 2 || videoRow.VideoDanmu != 1 || videoRow.VideoDuration != 60 {
		t.Errorf("视频行统计列 = %+v", videoRow)
	}

	if videoRow.DynContent != "" || videoRow.DynImageURL != "" {
		t.Errorf("视频行的动态列应为 NULL→零值, got %+v", videoRow)
	}

	if rows[1].DynContent != "first" || rows[1].DynLikeCount != 1 {
		t.Errorf("动态行 = %+v", rows[1])
	}

	// 仅视频 / 仅动态。
	rows, err = f.feed.ListAuthorWorks(ctx, app.AuthorFeedParams{
		AuthorID: 1, AuthorUsername: "alice", Sources: domain.BuildFeedSources(1), Limit: 10,
	})
	if err != nil || len(rows) != 1 {
		t.Fatalf("仅视频 = %d 行, %v", len(rows), err)
	}

	rows, err = f.feed.ListAuthorWorks(ctx, app.AuthorFeedParams{
		AuthorID: 1, AuthorUsername: "alice", Sources: domain.BuildFeedSources(2), Limit: 10,
	})
	if err != nil || len(rows) != 2 {
		t.Fatalf("仅动态 = %d 行, %v", len(rows), err)
	}

	total, err := f.feed.CountAuthorWorks(ctx, app.AuthorFeedCountParams{
		AuthorID: 1, AuthorUsername: "alice", Sources: sources,
	})
	if err != nil || total != 3 {
		t.Fatalf("CountAuthorWorks = %d, %v; want 3", total, err)
	}

	comments, err := f.feed.CommentCountsByDynamic(ctx, []uint{1, 2})
	if err != nil {
		t.Fatalf("CommentCountsByDynamic: %v", err)
	}

	if comments[1] != 1 || comments[2] != 0 {
		t.Fatalf("CommentCountsByDynamic = %+v", comments)
	}

	likedDynamics, err := f.feed.LikedDynamicIDs(ctx, 2, []uint{1, 2})
	if err != nil {
		t.Fatalf("LikedDynamicIDs: %v", err)
	}

	if !likedDynamics[1] || likedDynamics[2] {
		t.Fatalf("LikedDynamicIDs = %+v", likedDynamics)
	}

	likedVideos, err := f.feed.LikedVideoIDs(ctx, 2, []uint{1})
	if err != nil {
		t.Fatalf("LikedVideoIDs: %v", err)
	}

	if !likedVideos[1] {
		t.Fatalf("LikedVideoIDs = %+v", likedVideos)
	}
}

// TestFeedQueryOwnerViewIncludesUnpublished 作者本人视角可见未发布视频。
func TestFeedQueryOwnerViewIncludesUnpublished(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()

	if err := f.db.Session(&gorm.Session{SkipHooks: true}).Create(&po.Video{
		Model: po.Model{ID: 2}, Title: "draft", AuthorUsername: "alice",
		PartitionID: 1, Status: enum.VideoStatusReview,
	}).Error; err != nil {
		t.Fatalf("create draft video: %v", err)
	}

	params := app.AuthorFeedParams{
		AuthorID: 1, AuthorUsername: "alice", Sources: domain.BuildFeedSources(1), Limit: 10,
	}

	rows, err := f.feed.ListAuthorWorks(ctx, params)
	if err != nil || len(rows) != 1 {
		t.Fatalf("他人视角 = %d 行, %v; want 1", len(rows), err)
	}

	params.IsOwnerView = true

	rows, err = f.feed.ListAuthorWorks(ctx, params)
	if err != nil || len(rows) != 2 {
		t.Fatalf("本人视角 = %d 行, %v; want 2", len(rows), err)
	}
}

// TestAdminDynamicQueryList 覆盖管理端列表；带 username 过滤会 JOIN users，
// 原实现的裸排序列在此会触发列名歧义（见 report bug 条目），这里断言修复后可用。
func TestAdminDynamicQueryList(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()

	items, total, err := f.adminList.List(ctx, app.AdminDynamicListParams{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if total != 2 || len(items) != 2 {
		t.Fatalf("List = %d 项 total=%d", len(items), total)
	}

	if items[0].ID != 2 || items[0].Username != "alice" || items[0].Avatar != "a.png" {
		t.Fatalf("首项 = %+v, want 动态 2 + 作者资料", items[0])
	}

	items, total, err = f.adminList.List(ctx, app.AdminDynamicListParams{Username: "ali", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List with username: %v", err)
	}

	if total != 2 || len(items) != 2 || items[0].ID != 2 {
		t.Fatalf("按用户名过滤 = %+v total=%d", items, total)
	}

	items, total, err = f.adminList.List(ctx, app.AdminDynamicListParams{Keyword: "first", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List with keyword: %v", err)
	}

	if total != 1 || len(items) != 1 || items[0].ID != 1 {
		t.Fatalf("按关键字过滤 = %+v total=%d", items, total)
	}

	items, total, err = f.adminList.List(ctx, app.AdminDynamicListParams{UserID: 3, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List with userId: %v", err)
	}

	if total != 0 || len(items) != 0 {
		t.Fatalf("无匹配时应返回空列表, got %+v total=%d", items, total)
	}

	if items == nil {
		t.Error("空结果应返回非 nil 空切片（响应中渲染为 []）")
	}
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
