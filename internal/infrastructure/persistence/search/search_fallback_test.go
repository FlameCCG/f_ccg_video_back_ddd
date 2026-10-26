package search_test

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/search"

	domain "f_ccg_video_back_ddd/internal/domain/search"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/search"

	"gorm.io/gorm"
)

const (
	testAlice = "alice"
)

// 对应原 service/search_service/fallback.go（ES 不可用时的 MySQL 回退查询）。

// seedFallbackFixture 造 3 个已发布视频 + 1 个待审视频 + 3 个用户 + 关注关系。
func seedFallbackFixture(t *testing.T, db *gorm.DB) {
	t.Helper()

	session := db.Session(&gorm.Session{SkipHooks: true})

	tags := []po.Tag{
		{Model: po.Model{ID: 1}, Name: "教程"},
		{Model: po.Model{ID: 2}, Name: "vlog"},
	}
	for i := range tags {
		{
			err := session.Create(&tags[i]).Error
			require.NoError(t, err, "create tag failed: %v", err)
		}
	}

	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	videos := []po.Video{
		{
			Model:          po.Model{ID: 1, CreatedAt: base},
			Title:          "Go 教程 入门",
			AuthorUsername: testAlice,
			Status:         enum.VideoStatusPublished,
			Views:          10,
			DanmuCount:     5,
			Duration:       120,
			Tags:           []po.Tag{tags[0]},
		},
		{
			Model:          po.Model{ID: 2, CreatedAt: base.Add(time.Hour)},
			Title:          "无关标题",
			AuthorUsername: "教程君",
			Status:         enum.VideoStatusPublished,
			Views:          30,
			DanmuCount:     1,
			Duration:       60,
			Tags:           []po.Tag{tags[1]},
		},
		{
			Model:          po.Model{ID: 3, CreatedAt: base.Add(2 * time.Hour)},
			Title:          "教程",
			AuthorUsername: "bob",
			Status:         enum.VideoStatusPublished,
			Views:          20,
			DanmuCount:     9,
			Duration:       30,
		},
		{
			Model:          po.Model{ID: 4, CreatedAt: base},
			Title:          "教程 待审",
			AuthorUsername: "carol",
			Status:         enum.VideoStatusReview,
			Views:          999,
		},
	}
	for i := range videos {
		{
			err := session.Create(&videos[i]).Error
			require.NoError(t, err, "create video failed: %v", err)
		}
	}

	users := []po.User{
		{Model: po.Model{ID: 1}, Username: testAlice, Email: "a@e.com", Password: "x", Level: 3},
		{Model: po.Model{ID: 2}, Username: "alicia", Email: "b@e.com", Password: "x", Level: 1},
		{Model: po.Model{ID: 3}, Username: "bob", Email: "c@e.com", Password: "x", Level: 5},
	}
	for i := range users {
		{
			err := session.Create(&users[i]).Error
			require.NoError(t, err, "create user failed: %v", err)
		}
	}

	// alice 有 2 个粉丝，alicia 有 0 个。
	for _, focus := range []po.UserFocus{{UserID: 2, FocusUserID: 1}, {UserID: 3, FocusUserID: 1}} {
		{
			err := session.Create(&focus).Error
			require.NoError(t, err, "create focus failed: %v", err)
		}
	}
}

// TestSearchVideosOnlyReturnsPublishedAndMatchesTitleOrAuthor 只取已发布视频，
// 标题或作者名任一命中即入结果；总量为过滤后的行数。
func TestSearchVideosOnlyReturnsPublishedAndMatchesTitleOrAuthor(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedFallbackFixture(t, db)

	items, total, err := infra.NewSearchFallbackQuery(db).SearchVideos(context.Background(), app.Options{
		Keyword:   "教程",
		Page:      1,
		PageSize:  20,
		VideoSort: domain.VideoSortRelevance,
	})
	require.NoError(t, err, "SearchVideos: %v", err)

	// 待审视频（id=4）必须被过滤掉。
	require.False(t, total != 3 || len(items) != 3, "total = %d, items = %d", total, len(items))

	// 相关性排序的 CASE 优先级：title 精确(0) → title 前缀(1) → author 精确(2)
	// → author 前缀(3) → 其它(4)。
	// id=3 标题精确命中(0)；id=2 作者名「教程君」前缀命中(3)；
	// id=1 标题含「教程」但不以其开头、作者也不命中(4)。
	{
		got := videoIDs(items)
		require.True(t, reflect.DeepEqual(got, []uint{3, 2, 1}), "relevance order = %v, want [3 2 1]", got)
	}

	// 标签随 Preload("Tags") 一起回填；无标签时是空切片而非 nil。
	tagsByID := make(map[uint][]string, len(items))
	for _, item := range items {
		tagsByID[item.ID] = item.Tags
	}

	require.True(t, reflect.DeepEqual(tagsByID[1], []string{"教程"}), "tags[1] = %v, want [教程]", tagsByID[1])

	require.True(t, reflect.DeepEqual(tagsByID[2], []string{"vlog"}), "tags[2] = %v, want [vlog]", tagsByID[2])

	// id=3 没有标签，必须是空切片（响应里是 [] 而非 null）。
	{
		got := tagsByID[3]
		require.NotNil(t, got, "tags[3] = %v, want empty slice", got)
		require.Empty(t, got, "tags[3] = %v, want empty slice", got)
	}
}

// TestSearchVideosAppliesBusinessSortAndPaging 业务字段排序 + 分页偏移。
func TestSearchVideosAppliesBusinessSortAndPaging(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedFallbackFixture(t, db)
	query := infra.NewSearchFallbackQuery(db)

	tests := []struct {
		name  string
		sort  int
		order int
		want  []uint
	}{
		{name: "views desc", sort: domain.VideoSortViews, order: domain.SortOrderDesc, want: []uint{2, 3, 1}},
		{name: "views asc", sort: domain.VideoSortViews, order: domain.SortOrderAsc, want: []uint{1, 3, 2}},
		{name: "danmu desc", sort: domain.VideoSortDanmu, order: domain.SortOrderDesc, want: []uint{3, 1, 2}},
		{name: "duration asc", sort: domain.VideoSortDuration, order: domain.SortOrderAsc, want: []uint{3, 2, 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			items, _, err := query.SearchVideos(context.Background(), app.Options{
				Keyword:    "教程",
				Page:       1,
				PageSize:   20,
				VideoSort:  tt.sort,
				VideoOrder: tt.order,
			})
			require.NoError(t, err, "SearchVideos: %v", err)

			{
				got := videoIDs(items)
				require.True(t, reflect.DeepEqual(got, tt.want), "order = %v, want %v", got, tt.want)
			}
		})
	}

	// 第二页只剩一条（总量 3，页大小 2）。
	items, total, err := query.SearchVideos(context.Background(), app.Options{
		Keyword:    "教程",
		Page:       2,
		PageSize:   2,
		VideoSort:  domain.VideoSortViews,
		VideoOrder: domain.SortOrderDesc,
	})
	require.NoError(t, err, "SearchVideos: %v", err)

	require.False(t, total != 3 || len(items) != 1 || items[0].ID != 1, "page 2 = %+v (total %d)", videoIDs(items), total)
}

// TestSearchUsersAggregatesFollowerCountAndSorts 粉丝数由子查询聚合，
// 无粉丝时 COALESCE 成 0；支持按粉丝数/等级排序。
func TestSearchUsersAggregatesFollowerCountAndSorts(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedFallbackFixture(t, db)
	query := infra.NewSearchFallbackQuery(db)

	items, total, err := query.SearchUsers(context.Background(), app.Options{
		Keyword:   "ali",
		Page:      1,
		PageSize:  20,
		UserSort:  domain.UserSortFollowers,
		UserOrder: domain.SortOrderDesc,
	})
	require.NoError(t, err, "SearchUsers: %v", err)

	require.EqualValues(t, 2, total, "total = %d, items = %+v", total, items)
	require.Len(t, items, 2, "total = %d, items = %+v", total, items)

	require.EqualValues(t, 1, items[0].ID, "first user = %+v", items[0])
	require.EqualValues(t, 2, items[0].FollowerCount, "first user = %+v", items[0])

	require.EqualValues(t, 2, items[1].ID, "second user = %+v", items[1])
	require.EqualValues(t, 0, items[1].FollowerCount, "second user = %+v", items[1])

	// 按等级降序时 alice(level 3) 应排在 alicia(level 1) 之前——
	// 与按粉丝数排序的结果区分开，证明排序字段确实切换了。
	items, _, err = query.SearchUsers(context.Background(), app.Options{
		Keyword:   "ali",
		Page:      1,
		PageSize:  20,
		UserSort:  domain.UserSortLevel,
		UserOrder: domain.SortOrderAsc,
	})
	require.NoError(t, err, "SearchUsers: %v", err)

	require.Len(t, items, 2, "level asc order = %+v", items)
	require.EqualValues(t, 2, items[0].ID, "level asc order = %+v", items)
	require.EqualValues(t, 1, items[1].ID, "level asc order = %+v", items)
}

func TestCountUsersMatchesLikePattern(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedFallbackFixture(t, db)
	query := infra.NewSearchFallbackQuery(db)

	total, err := query.CountUsers(context.Background(), "  ali  ")
	require.NoError(t, err, "CountUsers: %v", err)

	require.EqualValues(t, 2, total, "total = %d, want 2", total)

	// 空白关键词不查库，直接返回 0。
	{
		total, err := query.CountUsers(context.Background(), "   ")
		require.NoError(t, err, "blank keyword: total = %d, err = %v", total, err)
		require.EqualValues(t, 0, total, "blank keyword: total = %d, err = %v", total, err)
	}
}

func TestVideoMetaByIDsReturnsAuthorAndCreatedAt(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedFallbackFixture(t, db)

	rows, err := infra.NewSearchFallbackQuery(db).VideoMetaByIDs(context.Background(), []uint{1, 3, 999})
	require.NoError(t, err, "VideoMetaByIDs: %v", err)

	require.Len(t, rows, 2, "rows = %+v, want 2 entries", rows)

	byID := make(map[uint]app.VideoSearchMeta, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}

	{
		got := byID[1]
		require.Equal(t, testAlice, got.AuthorUsername, "meta[1] = %+v", got)
		require.False(t, got.CreatedAt.IsZero(), "meta[1] = %+v", got)
	}

	// 空 id 集合不查库。
	{
		rows, err := infra.NewSearchFallbackQuery(db).VideoMetaByIDs(context.Background(), nil)
		require.NoError(t, err, "empty ids: rows = %+v, err = %v", rows, err)
		require.Nil(t, rows, "empty ids: rows = %+v, err = %v", rows, err)
	}
}

// TestSearchFallbackDegradesWithoutDB db 为 nil 时全部查询返回零值、不报错，
// 与原 global.DB == nil 的降级行为一致。
func TestSearchFallbackDegradesWithoutDB(t *testing.T) {
	t.Parallel()

	query := infra.NewSearchFallbackQuery(nil)
	require.False(t, query.Available(), "nil db must report unavailable")

	ctx := context.Background()
	opts := app.Options{Keyword: "x", Page: 1, PageSize: 20}

	{
		items, total, err := query.SearchVideos(ctx, opts)
		require.NoError(t, err, "SearchVideos = %+v, %d, %v", items, total, err)
		require.Nil(t, items, "SearchVideos = %+v, %d, %v", items, total, err)
		require.EqualValues(t, 0, total, "SearchVideos = %+v, %d, %v", items, total, err)
	}

	{
		items, total, err := query.SearchUsers(ctx, opts)
		require.NoError(t, err, "SearchUsers = %+v, %d, %v", items, total, err)
		require.Nil(t, items, "SearchUsers = %+v, %d, %v", items, total, err)
		require.EqualValues(t, 0, total, "SearchUsers = %+v, %d, %v", items, total, err)
	}

	{
		total, err := query.CountUsers(ctx, "x")
		require.NoError(t, err, "CountUsers = %d, %v", total, err)
		require.EqualValues(t, 0, total, "CountUsers = %d, %v", total, err)
	}

	{
		rows, err := query.VideoMetaByIDs(ctx, []uint{1})
		require.NoError(t, err, "VideoMetaByIDs = %+v, %v", rows, err)
		require.Nil(t, rows, "VideoMetaByIDs = %+v, %v", rows, err)
	}
}

// TestSearchFallbackSkipsBlankKeyword 空白关键词不查库（避免全表扫描）。
func TestSearchFallbackSkipsBlankKeyword(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedFallbackFixture(t, db)
	query := infra.NewSearchFallbackQuery(db)

	ctx := context.Background()
	opts := app.Options{Keyword: "   ", Page: 1, PageSize: 20}

	{
		items, total, err := query.SearchVideos(ctx, opts)
		require.NoError(t, err, "SearchVideos = %+v, %d, %v", items, total, err)
		require.Nil(t, items, "SearchVideos = %+v, %d, %v", items, total, err)
		require.EqualValues(t, 0, total, "SearchVideos = %+v, %d, %v", items, total, err)
	}

	{
		items, total, err := query.SearchUsers(ctx, opts)
		require.NoError(t, err, "SearchUsers = %+v, %d, %v", items, total, err)
		require.Nil(t, items, "SearchUsers = %+v, %d, %v", items, total, err)
		require.EqualValues(t, 0, total, "SearchUsers = %+v, %d, %v", items, total, err)
	}
}

// TestHotVideoTitlesSamplesPublishedByViews 热词回退按播放量取已发布视频标题样本。
func TestHotVideoTitlesSamplesPublishedByViews(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedFallbackFixture(t, db)

	rows, err := infra.NewHotKeywordQuery(db).HotVideoTitles(context.Background(), 10)
	require.NoError(t, err, "HotVideoTitles: %v", err)

	// 待审视频（views=999）不入样本；其余按播放量降序。
	want := []app.HotVideoTitle{
		{Title: "无关标题", Views: 30},
		{Title: "教程", Views: 20},
		{Title: "Go 教程 入门", Views: 10},
	}
	require.True(t, reflect.DeepEqual(rows, want), "rows = %+v, want %+v", rows, want)

	// 采样上限生效。
	{
		rows, err := infra.NewHotKeywordQuery(db).HotVideoTitles(context.Background(), 2)
		require.NoError(t, err, "limited sample = %+v, err = %v", rows, err)
		require.Len(t, rows, 2, "limited sample = %+v, err = %v", rows, err)
	}

	offline := infra.NewHotKeywordQuery(nil)
	require.False(t, offline.Available(), "nil db must report unavailable")

	{
		rows, err := offline.HotVideoTitles(context.Background(), 10)
		require.NoError(t, err, "offline = %+v, err = %v", rows, err)
		require.Nil(t, rows, "offline = %+v, err = %v", rows, err)
	}
}

// TestFollowingWithFriendFlagMarksMutualFollows @ 提及候选只含自己关注的人，
// 并标注互关与粉丝数。
func TestFollowingWithFriendFlagMarksMutualFollows(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedFallbackFixture(t, db)

	// user 2 关注 user 1；再让 user 1 关注 user 2 形成互关。
	{
		err := db.Session(&gorm.Session{SkipHooks: true}).
			Create(&po.UserFocus{UserID: 1, FocusUserID: 2}).Error
		require.NoError(t, err, "create mutual focus failed: %v", err)
	}

	users, err := infra.NewMentionQuery(db).FollowingWithFriendFlag(context.Background(), 2)
	require.NoError(t, err, "FollowingWithFriendFlag: %v", err)

	require.Len(t, users, 1, "users = %+v, want 1 entry", users)

	got := users[0]
	require.EqualValues(t, 1, got.ID, "mention user = %+v", got)
	require.Equal(t, testAlice, got.Username, "mention user = %+v", got)
	require.True(t, got.IsFriend, "mention user = %+v", got)
	require.EqualValues(t, 2, got.FollowerCount, "mention user = %+v", got)

	// 未关注任何人时返回空集合。
	users, err = infra.NewMentionQuery(db).FollowingWithFriendFlag(context.Background(), 1)
	require.NoError(t, err, "FollowingWithFriendFlag: %v", err)

	require.Len(t, users, 1, "reverse direction = %+v", users)
	require.EqualValues(t, 2, users[0].ID, "reverse direction = %+v", users)
	require.True(t, users[0].IsFriend, "reverse direction = %+v", users)

	{
		rows, err := infra.NewMentionQuery(nil).FollowingWithFriendFlag(context.Background(), 1)
		require.NoError(t, err, "offline = %+v, err = %v", rows, err)
		require.Nil(t, rows, "offline = %+v, err = %v", rows, err)
	}
}

// TestLoadDocumentsBuildsRelationStats 用户搜索文档带上粉丝数、单向关注名、互关好友名。
func TestLoadDocumentsBuildsRelationStats(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedFallbackFixture(t, db)

	session := db.Session(&gorm.Session{SkipHooks: true})

	// 造一个不会回关的 dave，用来覆盖「单向关注」分支
	// （fixture 里 alicia(2) 与 bob(3) 都已关注 alice(1)，回关即互关）。
	{
		err := session.Create(&po.User{
			Model:    po.Model{ID: 4},
			Username: "dave",
			Email:    "d@e.com",
			Password: "x",
		}).Error
		require.NoError(t, err, "create user failed: %v", err)
	}

	// alice(1) 关注 alicia(2)（互关）与 dave(4)（单向）。
	for _, focus := range []po.UserFocus{{UserID: 1, FocusUserID: 2}, {UserID: 1, FocusUserID: 4}} {
		{
			err := session.Create(&focus).Error
			require.NoError(t, err, "create focus failed: %v", err)
		}
	}

	docs, err := infra.NewUserDocumentQuery(db).LoadDocuments(context.Background(), []uint{1, 999})
	require.NoError(t, err, "LoadDocuments: %v", err)

	require.Len(t, docs, 1, "docs = %+v, want 1 entry (missing ids are skipped)", docs)

	got := docs[0]
	require.EqualValues(t, 1, got.ID, "doc = %+v", got)
	require.Equal(t, testAlice, got.Username, "doc = %+v", got)
	require.EqualValues(t, 2, got.FollowerCount, "doc = %+v", got)

	require.True(t, reflect.DeepEqual(got.FriendUsernames, []string{"alicia"}), "friends = %v, want [alicia]", got.FriendUsernames)

	require.True(t, reflect.DeepEqual(got.FollowUsernames, []string{"dave"}), "follows = %v, want [dave]", got.FollowUsernames)

	{
		docs, err := infra.NewUserDocumentQuery(nil).LoadDocuments(context.Background(), []uint{1})
		require.NoError(t, err, "offline = %+v, err = %v", docs, err)
		require.Nil(t, docs, "offline = %+v, err = %v", docs, err)
	}
}

// TestVectorSourceLoadReturnsTagsPartitionAndParts 向量索引源数据带上标签、分区名与分P标题。
func TestVectorSourceLoadReturnsTagsPartitionAndParts(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)

	{
		err := db.AutoMigrate(&po.VideoPartition{}, &po.Part{})
		require.NoError(t, err, "migrate failed: %v", err)
	}

	session := db.Session(&gorm.Session{SkipHooks: true})

	{
		err := session.Create(&po.VideoPartition{Model: po.Model{ID: 7}, Name: "科技"}).Error
		require.NoError(t, err, "create partition failed: %v", err)
	}

	tag := po.Tag{Model: po.Model{ID: 1}, Name: "教程"}
	{
		err := session.Create(&tag).Error
		require.NoError(t, err, "create tag failed: %v", err)
	}

	video := po.Video{
		Model:          po.Model{ID: 5},
		Title:          "Go 教程",
		Description:    "从零开始",
		Cover:          "uploads/cover.png",
		PartitionID:    7,
		Status:         enum.VideoStatusPublished,
		AuthorUsername: testAlice,
		Tags:           []po.Tag{tag},
	}
	{
		err := session.Create(&video).Error
		require.NoError(t, err, "create video failed: %v", err)
	}

	// 分P 按 sort_order 升序返回。
	parts := []po.Part{
		{Model: po.Model{ID: 2}, VideoID: 5, Title: "第二讲", SortOrder: 2},
		{Model: po.Model{ID: 1}, VideoID: 5, Title: "第一讲", SortOrder: 1},
	}
	for i := range parts {
		{
			err := session.Create(&parts[i]).Error
			require.NoError(t, err, "create part failed: %v", err)
		}
	}

	query := infra.NewVectorSourceQuery(db)

	source, err := query.Load(context.Background(), 5)
	require.NoError(t, err, "Load: %v", err)

	require.EqualValues(t, 5, source.ID, "source = %+v", source)
	require.Equal(t, "Go 教程", source.Title, "source = %+v", source)
	require.Equal(t, "科技", source.Partition, "source = %+v", source)

	require.Equal(t, enum.VideoStatusPublished, source.Status, "source = %+v", source)
	require.Equal(t, "uploads/cover.png", source.Cover, "source = %+v", source)

	require.True(t, reflect.DeepEqual(source.Tags, []string{"教程"}), "tags = %v", source.Tags)

	require.True(t, reflect.DeepEqual(source.PartTitles, []string{"第一讲", "第二讲"}), "part titles = %v", source.PartTitles)

	// 全量重建按页一次性预载相同的向量源数据，不能退化为先读 ID 再逐条 Load。
	page, err := query.PublishedAfter(context.Background(), 0, 200)
	require.NoError(t, err, "PublishedAfter: %v", err)

	require.Len(t, page, 1, "published page = %+v", page)
	require.Equal(t, page[0].ID, source.ID, "published page = %+v", page)
	require.Equal(t, page[0].Partition, source.Partition, "published page = %+v", page)

	require.True(t, reflect.DeepEqual(page[0].Tags, source.Tags), "published source = %+v, want %+v", page[0], source)
	require.True(t, reflect.DeepEqual(page[0].PartTitles, source.PartTitles), "published source = %+v, want %+v", page[0], source)

	// 视频不存在时返回可判定的哨兵错误（应用层据此删除向量文档）。
	{
		_, err := query.Load(context.Background(), 999)
		require.ErrorIs(t, err, app.ErrVectorSourceNotFound, "err = %v, want ErrVectorSourceNotFound", err)
	}

	offline := infra.NewVectorSourceQuery(nil)
	require.False(t, offline.Available(), "nil db must report unavailable")

	{
		_, err := offline.Load(context.Background(), 5)
		require.ErrorIs(t, err, app.ErrVectorSourceNotFound, "offline err = %v, want ErrVectorSourceNotFound", err)
	}
}

func videoIDs(items []app.VideoHit) []uint {
	ids := make([]uint, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	return ids
}
