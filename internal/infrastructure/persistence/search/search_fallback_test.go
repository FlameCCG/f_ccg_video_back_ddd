package search_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	app "f_ccg_video_back_ddd/internal/application/search"
	domain "f_ccg_video_back_ddd/internal/domain/search"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/search"

	"gorm.io/gorm"
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
		if err := session.Create(&tags[i]).Error; err != nil {
			t.Fatalf("create tag failed: %v", err)
		}
	}

	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	videos := []po.Video{
		{
			Model:          po.Model{ID: 1, CreatedAt: base},
			Title:          "Go 教程 入门",
			AuthorUsername: "alice",
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
		if err := session.Create(&videos[i]).Error; err != nil {
			t.Fatalf("create video failed: %v", err)
		}
	}

	users := []po.User{
		{Model: po.Model{ID: 1}, Username: "alice", Email: "a@e.com", Password: "x", Level: 3},
		{Model: po.Model{ID: 2}, Username: "alicia", Email: "b@e.com", Password: "x", Level: 1},
		{Model: po.Model{ID: 3}, Username: "bob", Email: "c@e.com", Password: "x", Level: 5},
	}
	for i := range users {
		if err := session.Create(&users[i]).Error; err != nil {
			t.Fatalf("create user failed: %v", err)
		}
	}

	// alice 有 2 个粉丝，alicia 有 0 个。
	for _, focus := range []po.UserFocus{{UserID: 2, FocusUserID: 1}, {UserID: 3, FocusUserID: 1}} {
		if err := session.Create(&focus).Error; err != nil {
			t.Fatalf("create focus failed: %v", err)
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
	if err != nil {
		t.Fatalf("SearchVideos: %v", err)
	}

	// 待审视频（id=4）必须被过滤掉。
	if total != 3 || len(items) != 3 {
		t.Fatalf("total = %d, items = %d", total, len(items))
	}

	// 相关性排序的 CASE 优先级：title 精确(0) → title 前缀(1) → author 精确(2)
	// → author 前缀(3) → 其它(4)。
	// id=3 标题精确命中(0)；id=2 作者名「教程君」前缀命中(3)；
	// id=1 标题含「教程」但不以其开头、作者也不命中(4)。
	if got := videoIDs(items); !reflect.DeepEqual(got, []uint{3, 2, 1}) {
		t.Fatalf("relevance order = %v, want [3 2 1]", got)
	}

	// 标签随 Preload("Tags") 一起回填；无标签时是空切片而非 nil。
	tagsByID := make(map[uint][]string, len(items))
	for _, item := range items {
		tagsByID[item.ID] = item.Tags
	}

	if !reflect.DeepEqual(tagsByID[1], []string{"教程"}) {
		t.Fatalf("tags[1] = %v, want [教程]", tagsByID[1])
	}

	if !reflect.DeepEqual(tagsByID[2], []string{"vlog"}) {
		t.Fatalf("tags[2] = %v, want [vlog]", tagsByID[2])
	}

	// id=3 没有标签，必须是空切片（响应里是 [] 而非 null）。
	if got := tagsByID[3]; got == nil || len(got) != 0 {
		t.Fatalf("tags[3] = %v, want empty slice", got)
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
			if err != nil {
				t.Fatalf("SearchVideos: %v", err)
			}

			if got := videoIDs(items); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("order = %v, want %v", got, tt.want)
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
	if err != nil {
		t.Fatalf("SearchVideos: %v", err)
	}

	if total != 3 || len(items) != 1 || items[0].ID != 1 {
		t.Fatalf("page 2 = %+v (total %d)", videoIDs(items), total)
	}
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
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}

	if total != 2 || len(items) != 2 {
		t.Fatalf("total = %d, items = %+v", total, items)
	}

	if items[0].ID != 1 || items[0].FollowerCount != 2 {
		t.Fatalf("first user = %+v", items[0])
	}

	if items[1].ID != 2 || items[1].FollowerCount != 0 {
		t.Fatalf("second user = %+v", items[1])
	}

	// 按等级降序时 alice(level 3) 应排在 alicia(level 1) 之前——
	// 与按粉丝数排序的结果区分开，证明排序字段确实切换了。
	items, _, err = query.SearchUsers(context.Background(), app.Options{
		Keyword:   "ali",
		Page:      1,
		PageSize:  20,
		UserSort:  domain.UserSortLevel,
		UserOrder: domain.SortOrderAsc,
	})
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}

	if len(items) != 2 || items[0].ID != 2 || items[1].ID != 1 {
		t.Fatalf("level asc order = %+v", items)
	}
}

func TestCountUsersMatchesLikePattern(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedFallbackFixture(t, db)
	query := infra.NewSearchFallbackQuery(db)

	total, err := query.CountUsers(context.Background(), "  ali  ")
	if err != nil {
		t.Fatalf("CountUsers: %v", err)
	}

	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}

	// 空白关键词不查库，直接返回 0。
	if total, err := query.CountUsers(context.Background(), "   "); err != nil || total != 0 {
		t.Fatalf("blank keyword: total = %d, err = %v", total, err)
	}
}

func TestVideoMetaByIDsReturnsAuthorAndCreatedAt(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedFallbackFixture(t, db)

	rows, err := infra.NewSearchFallbackQuery(db).VideoMetaByIDs(context.Background(), []uint{1, 3, 999})
	if err != nil {
		t.Fatalf("VideoMetaByIDs: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want 2 entries", rows)
	}

	byID := make(map[uint]app.VideoSearchMeta, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}

	if got := byID[1]; got.AuthorUsername != "alice" || got.CreatedAt.IsZero() {
		t.Fatalf("meta[1] = %+v", got)
	}

	// 空 id 集合不查库。
	if rows, err := infra.NewSearchFallbackQuery(db).VideoMetaByIDs(context.Background(), nil); err != nil || rows != nil {
		t.Fatalf("empty ids: rows = %+v, err = %v", rows, err)
	}
}

// TestSearchFallbackDegradesWithoutDB db 为 nil 时全部查询返回零值、不报错，
// 与原 global.DB == nil 的降级行为一致。
func TestSearchFallbackDegradesWithoutDB(t *testing.T) {
	t.Parallel()

	query := infra.NewSearchFallbackQuery(nil)
	if query.Available() {
		t.Fatal("nil db must report unavailable")
	}

	ctx := context.Background()
	opts := app.Options{Keyword: "x", Page: 1, PageSize: 20}

	if items, total, err := query.SearchVideos(ctx, opts); err != nil || items != nil || total != 0 {
		t.Fatalf("SearchVideos = %+v, %d, %v", items, total, err)
	}

	if items, total, err := query.SearchUsers(ctx, opts); err != nil || items != nil || total != 0 {
		t.Fatalf("SearchUsers = %+v, %d, %v", items, total, err)
	}

	if total, err := query.CountUsers(ctx, "x"); err != nil || total != 0 {
		t.Fatalf("CountUsers = %d, %v", total, err)
	}

	if rows, err := query.VideoMetaByIDs(ctx, []uint{1}); err != nil || rows != nil {
		t.Fatalf("VideoMetaByIDs = %+v, %v", rows, err)
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

	if items, total, err := query.SearchVideos(ctx, opts); err != nil || items != nil || total != 0 {
		t.Fatalf("SearchVideos = %+v, %d, %v", items, total, err)
	}

	if items, total, err := query.SearchUsers(ctx, opts); err != nil || items != nil || total != 0 {
		t.Fatalf("SearchUsers = %+v, %d, %v", items, total, err)
	}
}

// TestHotVideoTitlesSamplesPublishedByViews 热词回退按播放量取已发布视频标题样本。
func TestHotVideoTitlesSamplesPublishedByViews(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedFallbackFixture(t, db)

	rows, err := infra.NewHotKeywordQuery(db).HotVideoTitles(context.Background(), 10)
	if err != nil {
		t.Fatalf("HotVideoTitles: %v", err)
	}

	// 待审视频（views=999）不入样本；其余按播放量降序。
	want := []app.HotVideoTitle{
		{Title: "无关标题", Views: 30},
		{Title: "教程", Views: 20},
		{Title: "Go 教程 入门", Views: 10},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %+v, want %+v", rows, want)
	}

	// 采样上限生效。
	if rows, err := infra.NewHotKeywordQuery(db).HotVideoTitles(context.Background(), 2); err != nil || len(rows) != 2 {
		t.Fatalf("limited sample = %+v, err = %v", rows, err)
	}

	offline := infra.NewHotKeywordQuery(nil)
	if offline.Available() {
		t.Fatal("nil db must report unavailable")
	}

	if rows, err := offline.HotVideoTitles(context.Background(), 10); err != nil || rows != nil {
		t.Fatalf("offline = %+v, err = %v", rows, err)
	}
}

// TestFollowingWithFriendFlagMarksMutualFollows @ 提及候选只含自己关注的人，
// 并标注互关与粉丝数。
func TestFollowingWithFriendFlagMarksMutualFollows(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)
	seedFallbackFixture(t, db)

	// user 2 关注 user 1；再让 user 1 关注 user 2 形成互关。
	if err := db.Session(&gorm.Session{SkipHooks: true}).
		Create(&po.UserFocus{UserID: 1, FocusUserID: 2}).Error; err != nil {
		t.Fatalf("create mutual focus failed: %v", err)
	}

	users, err := infra.NewMentionQuery(db).FollowingWithFriendFlag(context.Background(), 2)
	if err != nil {
		t.Fatalf("FollowingWithFriendFlag: %v", err)
	}

	if len(users) != 1 {
		t.Fatalf("users = %+v, want 1 entry", users)
	}

	got := users[0]
	if got.ID != 1 || got.Username != "alice" || !got.IsFriend || got.FollowerCount != 2 {
		t.Fatalf("mention user = %+v", got)
	}

	// 未关注任何人时返回空集合。
	users, err = infra.NewMentionQuery(db).FollowingWithFriendFlag(context.Background(), 1)
	if err != nil {
		t.Fatalf("FollowingWithFriendFlag: %v", err)
	}

	if len(users) != 1 || users[0].ID != 2 || !users[0].IsFriend {
		t.Fatalf("reverse direction = %+v", users)
	}

	if rows, err := infra.NewMentionQuery(nil).FollowingWithFriendFlag(context.Background(), 1); err != nil || rows != nil {
		t.Fatalf("offline = %+v, err = %v", rows, err)
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
	if err := session.Create(&po.User{
		Model:    po.Model{ID: 4},
		Username: "dave",
		Email:    "d@e.com",
		Password: "x",
	}).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	// alice(1) 关注 alicia(2)（互关）与 dave(4)（单向）。
	for _, focus := range []po.UserFocus{{UserID: 1, FocusUserID: 2}, {UserID: 1, FocusUserID: 4}} {
		if err := session.Create(&focus).Error; err != nil {
			t.Fatalf("create focus failed: %v", err)
		}
	}

	docs, err := infra.NewUserDocumentQuery(db).LoadDocuments(context.Background(), []uint{1, 999})
	if err != nil {
		t.Fatalf("LoadDocuments: %v", err)
	}

	if len(docs) != 1 {
		t.Fatalf("docs = %+v, want 1 entry (missing ids are skipped)", docs)
	}

	got := docs[0]
	if got.ID != 1 || got.Username != "alice" || got.FollowerCount != 2 {
		t.Fatalf("doc = %+v", got)
	}

	if !reflect.DeepEqual(got.FriendUsernames, []string{"alicia"}) {
		t.Fatalf("friends = %v, want [alicia]", got.FriendUsernames)
	}

	if !reflect.DeepEqual(got.FollowUsernames, []string{"dave"}) {
		t.Fatalf("follows = %v, want [dave]", got.FollowUsernames)
	}

	if docs, err := infra.NewUserDocumentQuery(nil).LoadDocuments(context.Background(), []uint{1}); err != nil || docs != nil {
		t.Fatalf("offline = %+v, err = %v", docs, err)
	}
}

// TestVectorSourceLoadReturnsTagsPartitionAndParts 向量索引源数据带上标签、分区名与分P标题。
func TestVectorSourceLoadReturnsTagsPartitionAndParts(t *testing.T) {
	t.Parallel()

	db := newFallbackDB(t)

	if err := db.AutoMigrate(&po.VideoPartition{}, &po.Part{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	session := db.Session(&gorm.Session{SkipHooks: true})

	if err := session.Create(&po.VideoPartition{Model: po.Model{ID: 7}, Name: "科技"}).Error; err != nil {
		t.Fatalf("create partition failed: %v", err)
	}

	tag := po.Tag{Model: po.Model{ID: 1}, Name: "教程"}
	if err := session.Create(&tag).Error; err != nil {
		t.Fatalf("create tag failed: %v", err)
	}

	video := po.Video{
		Model:          po.Model{ID: 5},
		Title:          "Go 教程",
		Description:    "从零开始",
		Cover:          "uploads/cover.png",
		PartitionID:    7,
		Status:         enum.VideoStatusPublished,
		AuthorUsername: "alice",
		Tags:           []po.Tag{tag},
	}
	if err := session.Create(&video).Error; err != nil {
		t.Fatalf("create video failed: %v", err)
	}

	// 分P 按 sort_order 升序返回。
	parts := []po.Part{
		{Model: po.Model{ID: 2}, VideoID: 5, Title: "第二讲", SortOrder: 2},
		{Model: po.Model{ID: 1}, VideoID: 5, Title: "第一讲", SortOrder: 1},
	}
	for i := range parts {
		if err := session.Create(&parts[i]).Error; err != nil {
			t.Fatalf("create part failed: %v", err)
		}
	}

	query := infra.NewVectorSourceQuery(db)

	source, err := query.Load(context.Background(), 5)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if source.ID != 5 || source.Title != "Go 教程" || source.Partition != "科技" {
		t.Fatalf("source = %+v", source)
	}

	if source.Status != enum.VideoStatusPublished || source.Cover != "uploads/cover.png" {
		t.Fatalf("source = %+v", source)
	}

	if !reflect.DeepEqual(source.Tags, []string{"教程"}) {
		t.Fatalf("tags = %v", source.Tags)
	}

	if !reflect.DeepEqual(source.PartTitles, []string{"第一讲", "第二讲"}) {
		t.Fatalf("part titles = %v", source.PartTitles)
	}

	// 全量重建按页一次性预载相同的向量源数据，不能退化为先读 ID 再逐条 Load。
	page, err := query.PublishedAfter(context.Background(), 0, 200)
	if err != nil {
		t.Fatalf("PublishedAfter: %v", err)
	}
	if len(page) != 1 || page[0].ID != source.ID || page[0].Partition != source.Partition {
		t.Fatalf("published page = %+v", page)
	}
	if !reflect.DeepEqual(page[0].Tags, source.Tags) ||
		!reflect.DeepEqual(page[0].PartTitles, source.PartTitles) {
		t.Fatalf("published source = %+v, want %+v", page[0], source)
	}

	// 视频不存在时返回可判定的哨兵错误（应用层据此删除向量文档）。
	if _, err := query.Load(context.Background(), 999); !errors.Is(err, app.ErrVectorSourceNotFound) {
		t.Fatalf("err = %v, want ErrVectorSourceNotFound", err)
	}

	offline := infra.NewVectorSourceQuery(nil)
	if offline.Available() {
		t.Fatal("nil db must report unavailable")
	}

	if _, err := offline.Load(context.Background(), 5); !errors.Is(err, app.ErrVectorSourceNotFound) {
		t.Fatalf("offline err = %v, want ErrVectorSourceNotFound", err)
	}
}

func videoIDs(items []app.VideoHit) []uint {
	ids := make([]uint, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	return ids
}
