package social_test

import (
	"errors"
	"path/filepath"
	"testing"

	app "f_ccg_video_back_ddd/internal/application/social"
	socialdomain "f_ccg_video_back_ddd/internal/domain/social"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
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
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	if err := db.AutoMigrate(&po.User{}, &po.UserConf{}, &po.UserFocus{}, &po.Dynamic{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	// 跳过 AfterCreate（会写 user_confs / 默认收藏夹），本测自行控制 user_confs。
	session := db.Session(&gorm.Session{SkipHooks: true})

	users := []po.User{
		{Model: po.Model{ID: 1}, Username: "alice", Email: "a@example.com", Password: "x", Level: 3, Description: "A"},
		{Model: po.Model{ID: 2}, Username: "bob", Email: "b@example.com", Password: "x", Level: 2, Description: "B"},
		{Model: po.Model{ID: 3}, Username: "carol", Email: "c@example.com", Password: "x", Level: 1, Description: "C"},
	}
	for i := range users {
		if err := session.Create(&users[i]).Error; err != nil {
			t.Fatalf("create user failed: %v", err)
		}
	}

	if err := session.Create(&po.UserConf{UserID: 3}).Error; err != nil {
		t.Fatalf("create user conf failed: %v", err)
	}

	if err := session.Create(&po.UserFocus{UserID: 1, FocusUserID: 2}).Error; err != nil {
		t.Fatalf("create focus failed: %v", err)
	}

	if err := session.Create(&po.Dynamic{UserID: 1, Content: "hi"}).Error; err != nil {
		t.Fatalf("create dynamic failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}

	t.Cleanup(func() { _ = sqlDB.Close() })

	return infra.NewFocusRepository(db), infra.NewRelationQuery(db), infra.NewUserExistsQuery(db), db
}

func TestRelationQueryExistsAndRelation(t *testing.T) {
	_, query, _, _ := newFixture(t)
	ctx := t.Context()

	if !query.Exists(ctx, 1, 2) {
		t.Error("1 应已关注 2")
	}

	if query.Exists(ctx, 2, 1) {
		t.Error("2 未关注 1")
	}

	relation := query.Relation(ctx, 1, 2)
	if !relation.IsFocus || relation.IsFans || relation.IsMutual() {
		t.Errorf("relation(1,2) = %+v, want focus only", relation)
	}

	relation = query.Relation(ctx, 2, 1)
	if relation.IsFocus || !relation.IsFans {
		t.Errorf("relation(2,1) = %+v, want fans only", relation)
	}

	if got := query.Relation(ctx, 0, 2); got != (socialdomain.Relation{}) {
		t.Errorf("relation(0,2) = %+v, want zero", got)
	}
}

func TestRelationQueryCounts(t *testing.T) {
	_, query, _, _ := newFixture(t)
	ctx := t.Context()

	counts := query.Counts(ctx, 1, false)
	if counts.FocusCount != 1 || counts.FansCount != 0 || counts.DynamicCount != 0 {
		t.Errorf("counts(1,false) = %+v, want focus=1", counts)
	}

	counts = query.Counts(ctx, 1, true)
	if counts.DynamicCount != 1 {
		t.Errorf("counts(1,true).DynamicCount = %d, want 1", counts.DynamicCount)
	}

	counts = query.Counts(ctx, 2, false)
	if counts.FansCount != 1 {
		t.Errorf("counts(2,false).FansCount = %d, want 1", counts.FansCount)
	}

	if got := query.Counts(ctx, 0, false); got != (socialdomain.RelationCounts{}) {
		t.Errorf("counts(0) = %+v, want zero", got)
	}
}

func TestRelationQueryPrivacy(t *testing.T) {
	_, query, _, db := newFixture(t)
	ctx := t.Context()

	// 无 user_confs 行：COALESCE 缺省为公开。
	privacy, err := query.FollowPrivacy(ctx, 1)
	if err != nil {
		t.Fatalf("FollowPrivacy: %v", err)
	}

	if !privacy.Exists || !privacy.OpenFollow || !privacy.OpenFans {
		t.Errorf("privacy(1) = %+v, want all open", privacy)
	}

	if err := db.Model(&po.UserConf{}).Where("user_id = ?", 3).
		Updates(map[string]any{"open_follow": false, "open_fans": false}).Error; err != nil {
		t.Fatalf("update user conf: %v", err)
	}

	privacy, err = query.FollowPrivacy(ctx, 3)
	if err != nil {
		t.Fatalf("FollowPrivacy: %v", err)
	}

	if !privacy.Exists || privacy.OpenFollow || privacy.OpenFans {
		t.Errorf("privacy(3) = %+v, want closed", privacy)
	}

	action, err := query.ActionPrivacy(ctx, 1)
	if err != nil {
		t.Fatalf("ActionPrivacy: %v", err)
	}

	if !action.Exists || !action.OpenLikeVideo || !action.OpenCoinVideo {
		t.Errorf("action(1) = %+v, want all open", action)
	}

	if _, err := query.FollowPrivacy(ctx, 999); err == nil {
		t.Error("不存在的用户应返回错误")
	}
}

func TestRelationQueryLists(t *testing.T) {
	_, query, _, _ := newFixture(t)
	ctx := t.Context()

	rows, total, err := query.ListFocus(ctx, 1, 1, 10, "")
	if err != nil {
		t.Fatalf("ListFocus: %v", err)
	}

	if total != 1 || len(rows) != 1 || rows[0].ID != 2 || rows[0].Username != "bob" || rows[0].Level != 2 {
		t.Fatalf("ListFocus = %+v total=%d", rows, total)
	}

	rows, total, err = query.ListFans(ctx, 2, 1, 10, "")
	if err != nil {
		t.Fatalf("ListFans: %v", err)
	}

	if total != 1 || len(rows) != 1 || rows[0].ID != 1 {
		t.Fatalf("ListFans = %+v total=%d", rows, total)
	}

	rows, total, err = query.ListFocus(ctx, 1, 1, 10, "zzz")
	if err != nil {
		t.Fatalf("ListFocus with keyword: %v", err)
	}

	if total != 0 || len(rows) != 0 {
		t.Fatalf("关键字不匹配时应返回空列表，got %+v total=%d", rows, total)
	}

	if rows == nil {
		t.Error("空结果应返回非 nil 空切片（响应中渲染为 []）")
	}
}

func TestFocusRepositoryCreateAndDelete(t *testing.T) {
	repo, _, _, _ := newFixture(t)
	ctx := t.Context()

	focus, err := socialdomain.NewFocus(2, 3)
	if err != nil {
		t.Fatalf("NewFocus: %v", err)
	}

	created, err := repo.Create(ctx, focus)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if created == nil || created.ID() == 0 || created.CreatedAt().IsZero() || created.UpdatedAt().IsZero() {
		t.Fatalf("Create 应返回带主键和时间的实体，got %+v", created.Snapshot())
	}

	if created.UserID() != 2 || created.FocusUserID() != 3 {
		t.Fatalf("persisted relation = %d -> %d, want 2 -> 3", created.UserID(), created.FocusUserID())
	}

	if focus.ID() != 0 || !focus.CreatedAt().IsZero() || !focus.UpdatedAt().IsZero() {
		t.Fatal("Create 成功时也不应回写调用方持有的实体")
	}

	again, _ := socialdomain.NewFocus(2, 3)
	duplicated, err := repo.Create(ctx, again)
	if !errors.Is(err, socialdomain.ErrAlreadyFocused) {
		t.Fatalf("重复关注 err = %v, want %v", err, socialdomain.ErrAlreadyFocused)
	}

	if duplicated != nil || again.ID() != 0 {
		t.Fatal("重复关注失败时不应返回实体或改写入参")
	}

	affected, err := repo.Delete(ctx, 2, 3)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if affected != 1 {
		t.Errorf("Delete affected = %d, want 1", affected)
	}

	affected, err = repo.Delete(ctx, 2, 3)
	if err != nil {
		t.Fatalf("Delete again: %v", err)
	}

	if affected != 0 {
		t.Errorf("重复删除 affected = %d, want 0", affected)
	}
}

func TestUserExistsQuery(t *testing.T) {
	_, _, users, _ := newFixture(t)
	ctx := t.Context()

	exists, err := users.Exists(ctx, 1)
	if err != nil || !exists {
		t.Fatalf("Exists(1) = %v, %v; want true, nil", exists, err)
	}

	exists, err = users.Exists(ctx, 999)
	if err != nil || exists {
		t.Fatalf("Exists(999) = %v, %v; want false, nil", exists, err)
	}
}

// 编译期确认查询实现满足应用层端口。
var (
	_ app.RelationQuery   = (*infra.RelationQuery)(nil)
	_ app.UserExistsQuery = (*infra.UserExistsQuery)(nil)
)
