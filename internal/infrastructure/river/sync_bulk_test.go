package river

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/river/rule"
	"f_ccg_video_back_ddd/pkg/constants"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"

	elastic7 "github.com/olivere/elastic/v7"
	"github.com/siddontang/go-mysql-elasticsearch/elastic"
	"github.com/siddontang/go-mysql/schema"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRiverDoBulkAllowsSlowSuccessfulWrite(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// 模拟本地 ES 短暂慢写：超过旧的 3 秒期限，但最终能够成功。
		timer := time.NewTimer(4 * time.Second)
		defer timer.Stop()

		select {
		case <-timer.C:
			writeBulkSuccess(t, w)
		case <-req.Context().Done():
		}
	}))
	defer server.Close()

	r := newTestRiver(t, server)
	{
		err := r.doBulk(testUserBulkRequests())
		require.NoError(t, err, "slow successful bulk should finish without a timeout: %v", err)
	}
}

func TestRiverDoBulkWithRetryRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprint(w, `{"error":{"type":"es_rejected_execution_exception","reason":"busy"},"status":503}`)

			return
		}

		writeBulkSuccess(t, w)
	}))
	defer server.Close()

	r := newTestRiver(t, server)
	{
		err := r.doBulkWithRetry(testUserBulkRequests())
		require.NoError(t, err, "doBulkWithRetry failed: %v", err)
	}

	{
		got := calls.Load()
		require.EqualValues(t, 2, got, "bulk calls = %d, want 2", got)
	}
}

func TestRiverDoBulkWithRetryStopsAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, `{"error":{"type":"es_rejected_execution_exception","reason":"busy"},"status":503}`)
	}))
	defer server.Close()

	r := newTestRiver(t, server)

	err := r.doBulkWithRetry(testUserBulkRequests())
	require.Error(t, err, "doBulkWithRetry error = %v, want exhausted retry error", err)
	require.Contains(t, err.Error(), "still failing", "doBulkWithRetry error = %v, want exhausted retry error", err)

	{
		got := calls.Load()
		require.EqualValues(t, riverBulkMaxAttempts, got, "bulk calls = %d, want %d", got, riverBulkMaxAttempts)
	}
}

func TestRiverDoBulkWithRetryRejectsPermanentItemFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
			"took": 1,
			"errors": true,
			"items": [{
				"index": {
					"_index": "search_users",
					"_id": "users:2",
					"status": 429,
					"error": {
						"type": "es_rejected_execution_exception",
						"reason": "busy"
					}
				}
			}, {
				"index": {
					"_index": "search_users",
					"_id": "users:1",
					"status": 400,
					"error": {
						"type": "mapper_parsing_exception",
						"reason": "invalid field"
					}
				}
			}]
		}`)
	}))
	defer server.Close()

	r := newTestRiver(t, server)
	{
		err := r.doBulkWithRetry(testUserBulkRequests())
		require.Error(t, err, "doBulkWithRetry error = nil, want permanent item error")
	}

	{
		got := calls.Load()
		require.EqualValues(t, 1, got, "bulk calls = %d, want 1", got)
	}
}

func TestRiverDoBulkWithRetryRetriesTransientItemFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if calls.Add(1) == 1 {
			_, _ = fmt.Fprint(w, `{
				"took": 1,
				"errors": true,
				"items": [{
					"index": {
						"_index": "search_users",
						"_id": "users:1",
						"status": 429,
						"error": {
							"type": "es_rejected_execution_exception",
							"reason": "busy"
						}
					}
				}]
			}`)

			return
		}

		writeBulkSuccess(t, w)
	}))
	defer server.Close()

	r := newTestRiver(t, server)
	{
		err := r.doBulkWithRetry(testUserBulkRequests())
		require.NoError(t, err, "doBulkWithRetry failed: %v", err)
	}

	{
		got := calls.Load()
		require.EqualValues(t, 2, got, "bulk calls = %d, want 2", got)
	}
}

func TestRiverDoBulkWithRetryStopsDuringBackoff(t *testing.T) {
	t.Parallel()

	firstCall := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case <-firstCall:
		default:
			close(firstCall)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprint(w, `{"error":{"type":"es_rejected_execution_exception","reason":"busy"},"status":503}`)
	}))
	defer server.Close()

	r := newTestRiver(t, server)

	result := make(chan error, 1)
	go func() {
		result <- r.doBulkWithRetry(testUserBulkRequests())
	}()

	select {
	case <-firstCall:
	case <-time.After(time.Second):
		t.Fatal("bulk request did not start")
	}

	r.cancel()

	select {
	case err := <-result:
		require.Error(t, err, "doBulkWithRetry error = nil after cancellation")
	case <-time.After(time.Second):
		t.Fatal("doBulkWithRetry did not stop after cancellation")
	}
}

func TestIsRetryableESStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{name: "bad request", status: http.StatusBadRequest, want: false},
		{name: "conflict", status: http.StatusConflict, want: false},
		{name: "request timeout", status: http.StatusRequestTimeout, want: true},
		{name: "too early", status: http.StatusTooEarly, want: true},
		{name: "too many requests", status: http.StatusTooManyRequests, want: true},
		{name: "service unavailable", status: http.StatusServiceUnavailable, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			{
				got := isRetryableESStatus(tt.status)
				require.Equal(t, tt.want, got, "isRetryableESStatus(%d) = %t, want %t", tt.status, got, tt.want)
			}
		})
	}
}

// TestRiverUpdateUsersIndexesAfterImage users 表 UPDATE 直接使用 binlog after-image
// 组全文档，同时补齐关系冗余字段，不再回源用户主行。
func TestRiverUpdateUsersIndexesAfterImage(t *testing.T) {
	t.Parallel()

	db := newTestRiverDB(t)
	seedUserRelations(t, db)

	r := &River{ctx: context.Background(), db: db}
	ru := newTestRule(t, tableUsers, "id", "username", "description", "avatar", "level", "created_at", "updated_at")
	ru.Index = "custom_users"
	ru.Type = "_doc"
	ru.Pipeline = "normalize-users"
	ru.FieldMapping = map[string]string{"description": "bio"}

	// DB 中的 description 与 after-image 不同，断言基础字段以 binlog 事件为准。
	before := []any{int64(1), testBinlogUsername, "old desc", "old.png", int64(3), test20240101000000, "2024-01-02 00:00:00"}
	after := []any{int64(1), testBinlogUsername, "new desc from binlog", "new.png", int64(4), test20240101000000, "2024-01-03 00:00:00"}

	reqs, err := r.makeUpdateRequest(ru, [][]any{before, after})
	require.NoError(t, err, "makeUpdateRequest failed: %v", err)

	require.Len(t, reqs, 1, "requests = %d, want 1", len(reqs))

	req := reqs[0]
	require.Equal(t, elastic.ActionIndex, req.Action, "action = %q, want index", req.Action)

	require.Equal(t, "users:1", req.ID, "doc id = %q, want users:1", req.ID)

	require.Equal(t, "custom_users", req.Index, "rule metadata lost: index=%q type=%q pipeline=%q", req.Index, req.Type, req.Pipeline)
	require.Equal(t, "_doc", req.Type, "rule metadata lost: index=%q type=%q pipeline=%q", req.Index, req.Type, req.Pipeline)
	require.Equal(t, "normalize-users", req.Pipeline, "rule metadata lost: index=%q type=%q pipeline=%q", req.Index, req.Type, req.Pipeline)

	{
		got := req.Data["bio"]
		require.EqualValues(t, "new desc from binlog", got, "mapped bio = %v, want after-image value", got)
	}

	{
		_, exists := req.Data["description"]
		require.False(t, exists, "source description field should not remain after field mapping")
	}

	{
		got, _ := req.Data["follower_count"].(int64)
		require.EqualValues(t, 1, got, "follower_count = %d, want 1", got)
	}

	// 单向关注 celebrity、互关 fan：follow 列表排除互关好友
	{
		got := req.Data["follow_usernames"]
		require.True(t, reflect.DeepEqual(got, []string{"celebrity"}), "follow_usernames = %v, want [celebrity]", got)
	}

	{
		got := req.Data["friend_usernames"]
		require.True(t, reflect.DeepEqual(got, []string{"fan"}), "friend_usernames = %v, want [fan]", got)
	}
}

// TestRiverUpdateUsersDoesNotReloadMainRow 即使当前 DB 中已无该行，也按
// binlog after-image 产出幂等 index；后续 DELETE 事件会再删除文档。
func TestRiverUpdateUsersDoesNotReloadMainRow(t *testing.T) {
	t.Parallel()

	r := &River{ctx: context.Background(), db: newTestRiverDB(t)} // 不种入任何行
	ru := newTestRule(t, tableUsers, "id", "username", "description", "avatar", "level", "created_at", "updated_at")

	before := []any{int64(42), "gone", "d", testAPng, int64(1), nil, nil}
	after := []any{int64(42), "gone", "d2", testAPng, int64(2), nil, nil}

	reqs, err := r.makeUpdateRequest(ru, [][]any{before, after})
	require.NoError(t, err, "makeUpdateRequest failed: %v", err)

	require.Len(t, reqs, 1, "requests = %d, want 1", len(reqs))

	require.Equal(t, elastic.ActionIndex, reqs[0].Action, "request should index after-image, got action=%q data=%v", reqs[0].Action, reqs[0].Data)
	require.EqualValues(t, "d2", reqs[0].Data["description"], "request should index after-image, got action=%q data=%v", reqs[0].Action, reqs[0].Data)
}

// TestRiverUpdateUsersSkipsUnindexedBalanceChange 硬币/经验等非搜索字段更新
// 通常会伴随 updated_at 变化；这类事件不应重建用户文档或反查关系表。
func TestRiverUpdateUsersSkipsUnindexedBalanceChange(t *testing.T) {
	t.Parallel()

	db := newTestRiverDB(t)

	sqlDB, err := db.DB()
	require.NoError(t, err, "raw DB handle: %v", err)

	{
		err := sqlDB.Close()
		require.NoError(t, err, "close DB: %v", err)
	}

	r := &River{ctx: context.Background(), db: db}
	ru := newTestRule(t, tableUsers, "id", "username", "description", "avatar", "level", "coin_count", "created_at", "updated_at")
	ru.Filter = []string{"id", "username", "description", "avatar", "level", "created_at", "updated_at"}

	before := []any{int64(1), testBinlogUsername, "desc", testAPng, int64(1), 1.0, test20240101000000, "2024-01-02 00:00:00"}
	after := []any{int64(1), testBinlogUsername, "desc", testAPng, int64(1), 1.1, test20240101000000, "2024-01-03 00:00:00"}

	reqs, err := r.makeUpdateRequest(ru, [][]any{before, after})
	require.NoError(t, err, "makeUpdateRequest should not query DB: %v", err)

	require.Empty(t, reqs, "requests = %d, want 0 for unindexed balance-only update", len(reqs))
}

// TestRiverUpdateVideosIndexesAfterImage videos 表 UPDATE：直接用 binlog after-image
// 组全文档 index（不回源行）——未变更字段也完整写入，tags 冗余照常富化。
func TestRiverUpdateVideosIndexesAfterImage(t *testing.T) {
	t.Parallel()

	db := newTestRiverDB(t)
	seedVideoTags(t, db)

	r := &River{ctx: context.Background(), db: db}
	ru := newTestRule(t, "videos", "id", "title", "cover", "author_username", "created_at", "status", "views", "danmu_count", "duration")

	before := []any{int64(7), "Golang 入门", "cover.png", testAlice, test20240101000000, int64(1), int64(10), int64(0), int64(60)}
	after := []any{int64(7), "Golang 入门", "cover.png", testAlice, test20240101000000, int64(1), int64(11), int64(0), int64(60)} // 仅 views +1

	reqs, err := r.makeUpdateRequest(ru, [][]any{before, after})
	require.NoError(t, err, "makeUpdateRequest failed: %v", err)

	require.Len(t, reqs, 1, "requests = %d, want 1", len(reqs))

	req := reqs[0]
	require.Equal(t, elastic.ActionIndex, req.Action, "action = %q, want index", req.Action)

	require.Equal(t, "videos:7", req.ID, "doc id = %q, want videos:7", req.ID)

	{
		got, _ := req.Data["views"].(int64)
		require.EqualValues(t, 11, got, "views = %d, want 11", got)
	}

	{
		got, _ := req.Data["title"].(string)
		require.Equal(t, "Golang 入门", got, "title = %q, want unchanged field present in full doc", got)
	}

	{
		got := req.Data["tags"]
		require.True(t, reflect.DeepEqual(got, []string{"golang"}), "tags = %v, want [golang]", got)
	}
}

// TestRiverFullIndexRejectsEnrichmentQueryFailure 全文档覆盖前的派生字段查询
// 一旦失败必须中止，不能把标签/关系字段当作空值写入 ES。
func TestRiverFullIndexRejectsEnrichmentQueryFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		table  string
		cols   []string
		before []any
		after  []any
	}{
		{
			name:   "video tags",
			table:  "videos",
			cols:   []string{"id", "title", "cover", "author_username", "created_at", "status", "views", "danmu_count", "duration"},
			before: []any{int64(7), "old", "c.png", testAlice, test20240101000000, int64(1), int64(1), int64(0), int64(60)},
			after:  []any{int64(7), "new", "c.png", testAlice, test20240101000000, int64(1), int64(2), int64(0), int64(60)},
		},
		{
			name:   "user relations",
			table:  tableUsers,
			cols:   []string{"id", "username", "description", "avatar", "level", "created_at", "updated_at"},
			before: []any{int64(1), testBinlogUsername, "old", testAPng, int64(1), nil, nil},
			after:  []any{int64(1), testBinlogUsername, "new", testAPng, int64(2), nil, nil},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := newTestRiverDB(t)

			sqlDB, err := db.DB()
			require.NoError(t, err, "raw DB handle: %v", err)

			{
				err := sqlDB.Close()
				require.NoError(t, err, "close DB: %v", err)
			}

			r := &River{ctx: context.Background(), db: db}
			ru := newTestRule(t, tt.table, tt.cols...)

			{
				_, err := r.makeUpdateRequest(ru, [][]any{tt.before, tt.after})
				require.Error(t, err, "makeUpdateRequest should fail when enrichment DB query fails")
			}
		})
	}
}

// TestRiverUpdateBulkSendsIndexOnly UPDATE 构造出的请求经 ES bulk 发出时只含
// 覆盖式 index 动作——局部 update 在文档缺失时会确定性 404，该路径已不存在。
func TestRiverUpdateBulkSendsIndexOnly(t *testing.T) {
	t.Parallel()

	db := newTestRiverDB(t)
	seedUserRelations(t, db)

	var (
		mu     sync.Mutex
		bodies []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.Contains(r.URL.Path, "_bulk") {
			w.WriteHeader(http.StatusOK)

			return
		}

		body, _ := io.ReadAll(r.Body)

		mu.Lock()

		bodies = append(bodies, string(body))

		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		writeBulkSuccess(t, w)
	}))
	defer server.Close()

	r := newTestRiverWithDB(t, server, db)
	ru := newTestRule(t, tableUsers, "id", "username", "description", "avatar", "level", "created_at", "updated_at")

	before := []any{int64(1), testBinlogUsername, "old desc", "old.png", int64(3), test20240101000000, "2024-01-02 00:00:00"}
	after := []any{int64(1), testBinlogUsername, "new desc", "new.png", int64(4), test20240101000000, "2024-01-03 00:00:00"}

	reqs, err := r.makeUpdateRequest(ru, [][]any{before, after})
	require.NoError(t, err, "makeUpdateRequest failed: %v", err)

	{
		err := r.doBulkWithRetry(reqs)
		require.NoError(t, err, "doBulkWithRetry failed: %v", err)
	}

	mu.Lock()

	defer mu.Unlock()

	require.Len(t, bodies, 1, "bulk calls = %d, want 1", len(bodies))

	require.Contains(t, bodies[0], `{"index":`, "bulk body should only contain index actions, got %s", bodies[0])
	require.NotContains(t, bodies[0], `{"update":`, "bulk body should only contain index actions, got %s", bodies[0])

	require.Contains(t, bodies[0], `"username":"tester"`, "bulk body should contain the rebuilt full document, got %s", bodies[0])
}

// newTestRule 构造带表结构的最小同步规则：首列为单列主键。
func newTestRule(t *testing.T, table string, columns ...string) *rule.Rule {
	t.Helper()

	info := &schema.Table{Schema: testFCcgVideo, Name: table}

	for i, col := range columns {
		dataType := "varchar(255)"
		if strings.HasSuffix(col, "_at") {
			dataType = "datetime"
		}

		info.AddColumn(col, dataType, "", "")

		if i == 0 {
			info.PKColumns = append(info.PKColumns, i)
		}
	}

	return &rule.Rule{Schema: testFCcgVideo, Table: table, Index: "search_" + table, TableInfo: info}
}

// seedUserRelations 构造关系数据：fan(2) 与 tester(1) 互关，tester 单向关注 celebrity(3)。
func seedUserRelations(t *testing.T, db *gorm.DB) {
	t.Helper()

	{
		err := db.Exec(`INSERT INTO users (id, username, description, avatar, level, created_at, updated_at)
		VALUES (1, 'tester', 'desc from db', 'avatar.png', 3, '2024-01-01 00:00:00', '2024-01-02 00:00:00'),
		       (2, 'fan', 'fan desc', 'fan.png', 1, '2024-01-01 00:00:00', '2024-01-01 00:00:00'),
		       (3, 'celebrity', 'star desc', 'star.png', 9, '2024-01-01 00:00:00', '2024-01-01 00:00:00')`).Error
		require.NoError(t, err, "seed users failed: %v", err)
	}

	{
		err := db.Exec(`INSERT INTO user_focus (user_id, focus_user_id) VALUES (2, 1), (1, 2), (1, 3)`).Error
		require.NoError(t, err, "seed user_focus failed: %v", err)
	}
}

// seedVideoTags 给视频 7 挂一个标签 golang。
func seedVideoTags(t *testing.T, db *gorm.DB) {
	t.Helper()

	{
		err := db.Exec(`INSERT INTO tags (id, name) VALUES (1, 'golang')`).Error
		require.NoError(t, err, "seed tags failed: %v", err)
	}

	{
		err := db.Exec(`INSERT INTO video_tags (video_id, tag_id) VALUES (7, 1)`).Error
		require.NoError(t, err, "seed video_tags failed: %v", err)
	}
}

func newTestRiver(t *testing.T, server *httptest.Server) *River {
	t.Helper()

	return newTestRiverWithDB(t, server, nil)
}

func newTestRiverWithDB(t *testing.T, server *httptest.Server, db *gorm.DB) *River {
	t.Helper()

	client, err := elastic7.NewClient(
		elastic7.SetURL(server.URL),
		elastic7.SetHttpClient(server.Client()),
		elastic7.SetSniff(false),
		elastic7.SetHealthcheck(false),
	)
	require.NoError(t, err, "create ES client: %v", err)

	ctx, cancel := context.WithCancel(context.Background())

	t.Cleanup(func() {
		cancel()
		client.Stop()
	})

	return &River{ctx: ctx, cancel: cancel, es: client, db: db}
}

// newTestRiverDB 供回源重建路径使用的最小业务库（users + user_focus + tags + video_tags）。
func newTestRiverDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "river-test.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "open sqlite failed: %v", err)

	// Windows 下文件句柄不关闭会阻塞 TempDir 清理
	sqlDB, err := db.DB()
	require.NoError(t, err, "raw sqlite handle: %v", err)

	t.Cleanup(func() { _ = sqlDB.Close() })

	for _, ddl := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT, description TEXT, avatar TEXT, level INTEGER, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE user_focus (id INTEGER PRIMARY KEY, user_id INTEGER, focus_user_id INTEGER)`,
		`CREATE TABLE tags (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE video_tags (video_id INTEGER, tag_id INTEGER)`,
	} {
		{
			err := db.Exec(ddl).Error
			require.NoError(t, err, "create sqlite table failed: %v", err)
		}
	}

	return db
}

func testUserBulkRequests() []*elastic.BulkRequest {
	return []*elastic.BulkRequest{
		{
			Action: elastic.ActionIndex,
			Index:  constants.UserSearchIndex,
			ID:     "users:1",
			Data: map[string]any{
				"id":       uint(1),
				"username": testBinlogUsername,
			},
		},
	}
}

func writeBulkSuccess(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprint(w, `{
		"took": 1,
		"errors": false,
		"items": [{
			"index": {
				"_index": "search_users",
				"_id": "users:1",
				"status": 201
			}
		}]
	}`)
}

func TestRiverCallbackUnblocksWhenQueueConsumerStops(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())

	r := &River{ctx: ctx, cancel: cancel, syncCh: make(chan any, 1)}
	r.syncCh <- "full"

	done := make(chan error, 1)
	go func() { done <- r.enqueue("blocked") }()

	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled, "error=%v", err)
	case <-time.After(time.Second):
		t.Fatal("callback leaked after cancellation")
	}
}
