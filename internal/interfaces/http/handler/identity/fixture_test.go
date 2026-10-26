package identity

import (
	"context"
	"encoding/json"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/po"
	"f_ccg_video_back_ddd/pkg/res"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// testCustomBannerURL 自定义横幅 URL（与原 user_banner_setting_test.go 的常量一致）。
const testCustomBannerURL = "https://cdn.example.com/custom-banner.jpg"

// openTestDB 打开 sqlite 临时库并按需建表（测试不走 wire，见 architecture.md 第 8 节）。
//
// synchronous(OFF) 让建表 DDL 免去 fsync，AutoMigrate 明显更快。
func openTestDB(t *testing.T, models ...any) *gorm.DB {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "identity-handler-test.db") +
		"?_pragma=synchronous(OFF)&_pragma=journal_mode(MEMORY)"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		TranslateError:         true,
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err, "open sqlite failed: %v", err)

	{
		err = db.AutoMigrate(models...)
		require.NoError(t, err, "migrate failed: %v", err)
	}

	sqlDB, err := db.DB()
	require.NoError(t, err, "db.DB: %v", err)

	t.Cleanup(func() { _ = sqlDB.Close() })

	return db
}

// newTestContext 构造测试用 gin 上下文（无请求体，参数一律通过 c.Set("request") 注入）。
func newTestContext(method, target string) (*httptest.ResponseRecorder, *gin.Context) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), method, target, nil)

	return recorder, ctx
}

// decodeResponse 解析统一响应体。
func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder) res.Response {
	t.Helper()

	var response res.Response
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		require.NoError(t, err, "unmarshal response failed: %v", err)
	}

	return response
}

// decodeResponseData 把响应 data 再解回具体类型（先 marshal 再 unmarshal）。
func decodeResponseData(t *testing.T, response res.Response, out any) {
	t.Helper()

	payload, err := json.Marshal(response.Data)
	require.NoError(t, err, "marshal response data failed: %v", err)

	{
		err = json.Unmarshal(payload, out)
		require.NoError(t, err, "unmarshal response data failed: %v", err)
	}
}

// seedUserConf 直接写入用户配置行（跳过 GORM 钩子）。
func seedUserConf(t *testing.T, db *gorm.DB, conf po.UserConf) {
	t.Helper()

	{
		err := db.Session(&gorm.Session{SkipHooks: true}).Create(&conf).Error
		require.NoError(t, err, "seed user conf failed: %v", err)
	}
}

// seedBanner 写入一条横幅并返回自增 ID。
func seedBanner(t *testing.T, db *gorm.DB, banner *po.Banner) uint {
	t.Helper()

	{
		err := db.Create(banner).Error
		require.NoError(t, err, "seed banner failed: %v", err)
	}

	return banner.ID
}

func seedVisibleUserBanner(t *testing.T, db *gorm.DB, cover string) uint {
	t.Helper()
	return seedBanner(t, db, &po.Banner{Cover: cover, Show: true, Type: enum.BannerTypeUser})
}
