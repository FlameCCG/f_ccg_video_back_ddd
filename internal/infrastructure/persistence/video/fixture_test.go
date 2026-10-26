package video_test

import (
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"testing"

	"github.com/stretchr/testify/require"

	infra "f_ccg_video_back_ddd/internal/infrastructure/persistence/video"

	"gorm.io/gorm"
)

// testAuthorAlice 共享测试作者用户名（原 test_fixtures_test.go 的同名常量）。
const testAuthorAlice = "alice"

// fixture 一套用 sqlite 临时库装配的真实仓储与查询（测试不走 wire）。
type fixture struct {
	db          *gorm.DB
	videos      *infra.Repository
	partitions  *infra.PartitionRepository
	detail      *infra.DetailQuery
	adminVideos *infra.AdminVideoQuery
}

// session 返回跳过 GORM 钩子的会话（po.User 的 AfterCreate 会写 user_confs / 默认收藏夹）。
func (f *fixture) session() *gorm.DB {
	return f.db.Session(&gorm.Session{SkipHooks: true})
}

// createUsers 建用户（跳过钩子）。
func (f *fixture) createUsers(t *testing.T, users ...po.User) {
	t.Helper()

	session := f.session()

	for i := range users {
		{
			err := session.Create(&users[i]).Error
			require.NoError(t, err, "create user failed: %v", err)
		}
	}
}
