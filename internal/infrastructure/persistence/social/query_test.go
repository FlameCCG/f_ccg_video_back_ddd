package social_test

import (
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "f_ccg_video_back_ddd/internal/application/social"

	domain "f_ccg_video_back_ddd/internal/domain/social"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/social"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newFixture 用 sqlite 临时库装配真实仓储与查询（测试不走 wire）。
//
// 造 3 个用户：1 关注 2；2 未关注任何人；3 关闭了关注/粉丝公开开关。
func newFixture(t *testing.T) (*infra.FocusRepository, *infra.RelationQuery, *infra.UserExistsQuery, *gorm.DB) {
	t.Helper()

	// synchronous(OFF) 让建表 DDL 免去 fsync，AutoMigrate 明显更快。
	dsn := filepath.Join(t.TempDir(), "social-test.db") + "?_pragma=synchronous(OFF)&_pragma=journal_mode(MEMORY)"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		TranslateError:         true,
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err, "open sqlite failed: %v", err)

	{
		err := db.AutoMigrate(&po.User{}, &po.UserConf{}, &po.UserFocus{}, &po.Dynamic{})
		require.NoError(t, err, "migrate failed: %v", err)
	}

	// 跳过 AfterCreate（会写 user_confs / 默认收藏夹），本测自行控制 user_confs。
	session := db.Session(&gorm.Session{SkipHooks: true})

	users := []po.User{
		{Model: po.Model{ID: 1}, Username: "alice", Email: "a@example.com", Password: "x", Level: 3, Description: "A"},
		{Model: po.Model{ID: 2}, Username: "bob", Email: "b@example.com", Password: "x", Level: 2, Description: "B"},
		{Model: po.Model{ID: 3}, Username: "carol", Email: "c@example.com", Password: "x", Level: 1, Description: "C"},
	}
	for i := range users {
		{
			err := session.Create(&users[i]).Error
			require.NoError(t, err, "create user failed: %v", err)
		}
	}

	{
		err := session.Create(&po.UserConf{UserID: 3}).Error
		require.NoError(t, err, "create user conf failed: %v", err)
	}

	{
		err := session.Create(&po.UserFocus{UserID: 1, FocusUserID: 2}).Error
		require.NoError(t, err, "create focus failed: %v", err)
	}

	{
		err := session.Create(&po.Dynamic{UserID: 1, Content: "hi"}).Error
		require.NoError(t, err, "create dynamic failed: %v", err)
	}

	sqlDB, err := db.DB()
	require.NoError(t, err, "db.DB: %v", err)

	t.Cleanup(func() { _ = sqlDB.Close() })

	return infra.NewFocusRepository(db), infra.NewRelationQuery(db), infra.NewUserExistsQuery(db), db
}

func TestRelationQueryExistsAndRelation(t *testing.T) {
	t.Parallel()

	_, query, _, _ := newFixture(t)
	ctx := t.Context()

	assert.True(t, query.Exists(ctx, 1, 2), "1 应已关注 2")

	assert.False(t, query.Exists(ctx, 2, 1), "2 未关注 1")

	relation := query.Relation(ctx, 1, 2)
	assert.False(t, !relation.IsFocus || relation.IsFans || relation.IsMutual(), "relation(1,2) = %+v, want focus only", relation)

	relation = query.Relation(ctx, 2, 1)
	assert.False(t, relation.IsFocus || !relation.IsFans, "relation(2,1) = %+v, want fans only", relation)

	{
		got := query.Relation(ctx, 0, 2)
		assert.Equal(t, (domain.Relation{}), got, "relation(0,2) = %+v, want zero", got)
	}
}

func TestRelationQueryCounts(t *testing.T) {
	t.Parallel()

	_, query, _, _ := newFixture(t)
	ctx := t.Context()

	counts := query.Counts(ctx, 1, false)
	assert.False(t, counts.FocusCount != 1 || counts.FansCount != 0 || counts.DynamicCount != 0, "counts(1,false) = %+v, want focus=1", counts)

	counts = query.Counts(ctx, 1, true)
	assert.EqualValues(t, 1, counts.DynamicCount, "counts(1,true).DynamicCount = %d, want 1", counts.DynamicCount)

	counts = query.Counts(ctx, 2, false)
	assert.EqualValues(t, 1, counts.FansCount, "counts(2,false).FansCount = %d, want 1", counts.FansCount)

	{
		got := query.Counts(ctx, 0, false)
		assert.Equal(t, (domain.RelationCounts{}), got, "counts(0) = %+v, want zero", got)
	}
}

func TestRelationQueryPrivacy(t *testing.T) {
	t.Parallel()

	_, query, _, db := newFixture(t)
	ctx := t.Context()

	// 无 user_confs 行：COALESCE 缺省为公开。
	privacy, err := query.FollowPrivacy(ctx, 1)
	require.NoError(t, err, "FollowPrivacy: %v", err)

	assert.False(t, !privacy.Exists || !privacy.OpenFollow || !privacy.OpenFans, "privacy(1) = %+v, want all open", privacy)

	{
		err := db.Model(&po.UserConf{}).Where("user_id = ?", 3).
			Updates(map[string]any{"open_follow": false, "open_fans": false}).Error
		require.NoError(t, err, "update user conf: %v", err)
	}

	privacy, err = query.FollowPrivacy(ctx, 3)
	require.NoError(t, err, "FollowPrivacy: %v", err)

	assert.False(t, !privacy.Exists || privacy.OpenFollow || privacy.OpenFans, "privacy(3) = %+v, want closed", privacy)

	action, err := query.ActionPrivacy(ctx, 1)
	require.NoError(t, err, "ActionPrivacy: %v", err)

	assert.False(t, !action.Exists || !action.OpenLikeVideo || !action.OpenCoinVideo, "action(1) = %+v, want all open", action)

	{
		_, err := query.FollowPrivacy(ctx, 999)
		assert.Error(t, err, "不存在的用户应返回错误")
	}
}

func TestRelationQueryLists(t *testing.T) {
	t.Parallel()

	_, query, _, _ := newFixture(t)
	ctx := t.Context()

	rows, total, err := query.ListFocus(ctx, 1, 1, 10, "")
	require.NoError(t, err, "ListFocus: %v", err)

	require.EqualValues(t, 1, total, "ListFocus = %+v total=%d", rows, total)
	require.Len(t, rows, 1, "ListFocus = %+v total=%d", rows, total)
	require.EqualValues(t, 2, rows[0].ID, "ListFocus = %+v total=%d", rows, total)
	require.Equal(t, "bob", rows[0].Username, "ListFocus = %+v total=%d", rows, total)
	require.Equal(t, 2, rows[0].Level, "ListFocus = %+v total=%d", rows, total)

	rows, total, err = query.ListFans(ctx, 2, 1, 10, "")
	require.NoError(t, err, "ListFans: %v", err)

	require.EqualValues(t, 1, total, "ListFans = %+v total=%d", rows, total)
	require.Len(t, rows, 1, "ListFans = %+v total=%d", rows, total)
	require.EqualValues(t, 1, rows[0].ID, "ListFans = %+v total=%d", rows, total)

	rows, total, err = query.ListFocus(ctx, 1, 1, 10, "zzz")
	require.NoError(t, err, "ListFocus with keyword: %v", err)

	require.EqualValues(t, 0, total, "关键字不匹配时应返回空列表，got %+v total=%d", rows, total)
	require.Empty(t, rows, "关键字不匹配时应返回空列表，got %+v total=%d", rows, total)

	assert.NotNil(t, rows, "空结果应返回非 nil 空切片（响应中渲染为 []）")
}

func TestFocusRepositoryCreateAndDelete(t *testing.T) {
	t.Parallel()

	repo, _, _, _ := newFixture(t)
	ctx := t.Context()

	focus, err := domain.NewFocus(2, 3)
	require.NoError(t, err, "NewFocus: %v", err)

	{
		err := repo.Create(ctx, focus)
		require.NoError(t, err, "Create: %v", err)
	}

	assert.NotEqualValues(t, 0, focus.ID, "Create 应回填自增 ID")

	again, _ := domain.NewFocus(2, 3)
	{
		err := repo.Create(ctx, again)
		require.ErrorIs(t, err, domain.ErrAlreadyFocused, "重复关注 err = %v, want %v", err, domain.ErrAlreadyFocused)
	}

	affected, err := repo.Delete(ctx, 2, 3)
	require.NoError(t, err, "Delete: %v", err)

	assert.EqualValues(t, 1, affected, "Delete affected = %d, want 1", affected)

	affected, err = repo.Delete(ctx, 2, 3)
	require.NoError(t, err, "Delete again: %v", err)

	assert.EqualValues(t, 0, affected, "重复删除 affected = %d, want 0", affected)
}

func TestUserExistsQuery(t *testing.T) {
	t.Parallel()

	_, _, users, _ := newFixture(t)
	ctx := t.Context()

	exists, err := users.Exists(ctx, 1)
	require.NoError(t, err, "Exists(1) = %v, %v; want true, nil", exists, err)
	require.True(t, exists, "Exists(1) = %v, %v; want true, nil", exists, err)

	exists, err = users.Exists(ctx, 999)
	require.NoError(t, err, "Exists(999) = %v, %v; want false, nil", exists, err)
	require.False(t, exists, "Exists(999) = %v, %v; want false, nil", exists, err)
}

// 编译期确认查询实现满足应用层端口。
var (
	_ app.RelationQuery   = (*infra.RelationQuery)(nil)
	_ app.UserExistsQuery = (*infra.UserExistsQuery)(nil)
)
