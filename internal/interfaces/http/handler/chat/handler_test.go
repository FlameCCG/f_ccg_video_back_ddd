package chat

import (
	"context"
	"encoding/json"
	"f_ccg_video_back_ddd/pkg/res"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	chatapp "f_ccg_video_back_ddd/internal/application/chat"

	domain "f_ccg_video_back_ddd/internal/domain/chat"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 对应原 api/chat_api（原目录无单测，此处按处理器逐个补齐）。
//
// 处理器持有的是具体类型 *chatapp.Service，因此这里注入 stub 端口来驱动它：
// 测试不走 wire，也不依赖真实 Mongo/MySQL/Redis（见 architecture.md 第 8 节）。

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

// newTestContext 构造测试用 gin 上下文（参数一律通过 c.Set("request") 注入）。
func newTestContext(method, target string) (*httptest.ResponseRecorder, *gin.Context) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), method, target, nil)

	return recorder, ctx
}

// decodeResponse 解析统一响应包装。
func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder) res.Response {
	t.Helper()

	require.Equal(t, http.StatusOK, recorder.Code, "http status = %d, want 200", recorder.Code)

	var body res.Response
	{
		err := json.Unmarshal(recorder.Body.Bytes(), &body)
		require.NoError(t, err, "unmarshal response failed: %v (%s)", err, recorder.Body.String())
	}

	return body
}

// newHandler 造一个由 stub 端口驱动的处理器（hub 为 nil，WS 用例单独处理）。
func newHandler(repo *stubRepository, query *stubQuery) *Handler {
	service := chatapp.NewService(repo, query, &stubFollows{following: true}, nil, zap.NewNop())

	return NewHandler(service, nil)
}

// ---------------- ConversationListView ----------------

func TestConversationListViewRequiresLogin(t *testing.T) {
	t.Parallel()

	recorder, ctx := newTestContext(http.MethodGet, "/v1/common/chat/conversations")
	ctx.Set("request", ConversationListReq{Page: 1, PageSize: 20})

	newHandler(&stubRepository{available: true}, &stubQuery{}).ConversationListView(ctx)

	body := decodeResponse(t, recorder)
	require.EqualValues(t, res.FailServiceCode, body.Code, "body = %+v", body)
	require.Equal(t, testLoginRequired, body.Msg, "body = %+v", body)
}

// TestConversationListViewReturnsListEnvelope 成功响应是 {list, total} 包装，
// 且分页参数原样透给应用层。
func TestConversationListViewReturnsListEnvelope(t *testing.T) {
	t.Parallel()

	query := &stubQuery{
		conversations: []chatapp.ConversationItem{{ID: "1_2", PeerID: 2, Unread: 3, UpdatedAt: 1700000000}},
		total:         7,
	}

	recorder, ctx := newTestContext(http.MethodGet, "/v1/common/chat/conversations")
	ctx.Set("uid", "1")
	ctx.Set("request", ConversationListReq{Page: 2, PageSize: 15})

	newHandler(&stubRepository{available: true}, query).ConversationListView(ctx)

	body := decodeResponse(t, recorder)
	require.EqualValues(t, res.SuccessCode, body.Code, "body = %+v", body)
	require.Equal(t, "成功", body.Msg, "body = %+v", body)

	data, ok := body.Data.(map[string]any)
	require.True(t, ok, "data = %T, want object", body.Data)

	{
		got := data["total"]
		require.InDelta(t, float64(7), got, 1e-9, "total = %v, want 7", got)
	}

	list, ok := data["list"].([]any)
	require.True(t, ok, "list = %v", data["list"])
	require.Len(t, list, 1, "list = %v", data["list"])

	item, ok := list[0].(map[string]any)
	require.True(t, ok, "list[0] = %T", list[0])

	// 响应字段名属于外部契约。
	for _, key := range []string{"id", "peerId", "unread", "updatedAt"} {
		{
			_, exists := item[key]
			require.True(t, exists, "missing field %q in %v", key, item)
		}
	}

	require.Equal(t, 2, query.lastPage, "paging = (%d, %d), want (2, 15)", query.lastPage, query.lastPageSize)
	require.Equal(t, 15, query.lastPageSize, "paging = (%d, %d), want (2, 15)", query.lastPage, query.lastPageSize)
}

// TestConversationListViewSurfacesServiceError 服务层错误文案原样返回给前端。
func TestConversationListViewSurfacesServiceError(t *testing.T) {
	t.Parallel()

	recorder, ctx := newTestContext(http.MethodGet, "/v1/common/chat/conversations")
	ctx.Set("uid", "1")
	ctx.Set("request", ConversationListReq{})

	newHandler(&stubRepository{available: false}, &stubQuery{}).ConversationListView(ctx)

	body := decodeResponse(t, recorder)
	require.EqualValues(t, res.FailServiceCode, body.Code, "body = %+v", body)
	require.Equal(t, body.Msg, domain.ErrStoreUnavailable.Error(), "body = %+v", body)
}

// ---------------- CreateConversationView ----------------

func TestCreateConversationViewRequiresLogin(t *testing.T) {
	t.Parallel()

	recorder, ctx := newTestContext(http.MethodPost, "/v1/common/chat/conversations")
	ctx.Set("request", CreateConversationReq{PeerID: 2})

	newHandler(&stubRepository{available: true}, &stubQuery{}).CreateConversationView(ctx)

	{
		body := decodeResponse(t, recorder)
		require.Equal(t, testLoginRequired, body.Msg, "body = %+v", body)
	}
}

func TestCreateConversationViewReturnsItem(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := &stubRepository{
		available: true,
		conversation: &domain.Conversation{
			ID:        "1_2",
			MemberA:   domain.Member{UserID: 1, Unread: 4},
			MemberB:   domain.Member{UserID: 2, Unread: 1},
			CreatedAt: created,
			UpdatedAt: created,
		},
	}

	recorder, ctx := newTestContext(http.MethodPost, "/v1/common/chat/conversations")
	ctx.Set("uid", "1")
	ctx.Set("request", CreateConversationReq{PeerID: 2})

	newHandler(repo, &stubQuery{}).CreateConversationView(ctx)

	body := decodeResponse(t, recorder)
	require.EqualValues(t, res.SuccessCode, body.Code, "body = %+v", body)
	require.Equal(t, "ok", body.Msg, "body = %+v", body)

	item, ok := body.Data.(map[string]any)
	require.True(t, ok, "data = %T, want object", body.Data)

	require.EqualValues(t, "1_2", item["id"], "item = %v", item)
	require.InDelta(t, float64(2), item["peerId"], 1e-9, "item = %v", item)
	require.InDelta(t, float64(4), item["unread"], 1e-9, "item = %v", item)

	// 空会话没有 lastMessage（omitempty 省略该字段）。
	{
		_, exists := item["lastMessage"]
		require.False(t, exists, "empty conversation must omit lastMessage: %v", item)
	}
}

// TestCreateConversationViewRejectsSelf 自己给自己建会话的文案与其它路径不同。
func TestCreateConversationViewRejectsSelf(t *testing.T) {
	t.Parallel()

	recorder, ctx := newTestContext(http.MethodPost, "/v1/common/chat/conversations")
	ctx.Set("uid", "5")
	ctx.Set("request", CreateConversationReq{PeerID: 5})

	newHandler(&stubRepository{available: true}, &stubQuery{}).CreateConversationView(ctx)

	{
		body := decodeResponse(t, recorder)
		require.Equal(t, body.Msg, domain.ErrSelfConversation.Error(), "body = %+v", body)
	}
}

// ---------------- DeleteConversationView ----------------

func TestDeleteConversationViewRequiresLogin(t *testing.T) {
	t.Parallel()

	recorder, ctx := newTestContext(http.MethodDelete, "/v1/common/chat/conversations")
	ctx.Set("request", DeleteConversationReq{PeerID: 2})

	newHandler(&stubRepository{available: true}, &stubQuery{}).DeleteConversationView(ctx)

	{
		body := decodeResponse(t, recorder)
		require.Equal(t, testLoginRequired, body.Msg, "body = %+v", body)
	}
}

func TestDeleteConversationViewReturnsSuccessMessage(t *testing.T) {
	t.Parallel()

	repo := &stubRepository{available: true}

	recorder, ctx := newTestContext(http.MethodDelete, "/v1/common/chat/conversations")
	ctx.Set("uid", "9")
	ctx.Set("request", DeleteConversationReq{PeerID: 3})

	newHandler(repo, &stubQuery{}).DeleteConversationView(ctx)

	body := decodeResponse(t, recorder)
	require.EqualValues(t, res.SuccessCode, body.Code, "body = %+v", body)
	require.Equal(t, "删除成功", body.Msg, "body = %+v", body)

	require.EqualValues(t, 9, repo.hiddenFor, "hidden for (%d, %d), want (9, 3)", repo.hiddenFor, repo.hiddenPeer)
	require.EqualValues(t, 3, repo.hiddenPeer, "hidden for (%d, %d), want (9, 3)", repo.hiddenFor, repo.hiddenPeer)
}

// ---------------- MessageListView ----------------

func TestMessageListViewRequiresLogin(t *testing.T) {
	t.Parallel()

	recorder, ctx := newTestContext(http.MethodGet, "/v1/common/chat/messages")
	ctx.Set("request", MessageListReq{PeerID: 2})

	newHandler(&stubRepository{available: true}, &stubQuery{}).MessageListView(ctx)

	{
		body := decodeResponse(t, recorder)
		require.Equal(t, testLoginRequired, body.Msg, "body = %+v", body)
	}
}

// TestMessageListViewReturnsCursorEnvelope 响应含 list / hasMore（无更多时省略 nextBefore）。
func TestMessageListViewReturnsCursorEnvelope(t *testing.T) {
	t.Parallel()

	query := &stubQuery{messages: []chatapp.MessageItem{
		{ID: "652f1a2b3c4d5e6f70819202", ConversationID: "1_2", SenderID: 1, ReceiverID: 2, Type: domain.MessageTypeText, Text: "b"},
		{ID: "652f1a2b3c4d5e6f70819201", ConversationID: "1_2", SenderID: 2, ReceiverID: 1, Type: domain.MessageTypeText, Text: "a"},
	}}

	recorder, ctx := newTestContext(http.MethodGet, "/v1/common/chat/messages")
	ctx.Set("uid", "1")
	ctx.Set("request", MessageListReq{PeerID: 2})

	newHandler(&stubRepository{available: true}, query).MessageListView(ctx)

	body := decodeResponse(t, recorder)
	require.EqualValues(t, res.SuccessCode, body.Code, "body = %+v", body)

	data, ok := body.Data.(map[string]any)
	require.True(t, ok, "data = %T, want object", body.Data)

	require.EqualValues(t, false, data["hasMore"], "hasMore = %v, want false", data["hasMore"])

	{
		_, exists := data["nextBefore"]
		require.False(t, exists, "nextBefore must be omitted when there is no more data: %v", data)
	}

	list, ok := data["list"].([]any)
	require.True(t, ok, "list = %v", data["list"])
	require.Len(t, list, 2, "list = %v", data["list"])

	// 返回倒序（旧→新）：最早的一条在前。
	first, ok := list[0].(map[string]any)
	require.True(t, ok, "list[0] = %T", list[0])

	require.EqualValues(t, "a", first["text"], "messages must be oldest-first, got %v", first["text"])

	for _, key := range []string{"id", "conversationId", "senderId", "receiverId", "type", "createdAt"} {
		{
			_, exists := first[key]
			require.True(t, exists, "missing field %q in %v", key, first)
		}
	}
}

// TestMessageListViewRejectsInvalidCursor 非 24 位十六进制游标返回「before 参数无效」。
func TestMessageListViewRejectsInvalidCursor(t *testing.T) {
	t.Parallel()

	recorder, ctx := newTestContext(http.MethodGet, "/v1/common/chat/messages")
	ctx.Set("uid", "1")
	ctx.Set("request", MessageListReq{PeerID: 2, Before: "not-a-valid-object-id"})

	newHandler(&stubRepository{available: true}, &stubQuery{}).MessageListView(ctx)

	{
		body := decodeResponse(t, recorder)
		require.Equal(t, body.Msg, domain.ErrInvalidCursor.Error(), "body = %+v", body)
	}
}

// ---------------- MarkReadView ----------------

func TestMarkReadViewRequiresLogin(t *testing.T) {
	t.Parallel()

	recorder, ctx := newTestContext(http.MethodPut, "/v1/common/chat/read")
	ctx.Set("request", MarkReadReq{PeerID: 2})

	newHandler(&stubRepository{available: true}, &stubQuery{}).MarkReadView(ctx)

	{
		body := decodeResponse(t, recorder)
		require.Equal(t, testLoginRequired, body.Msg, "body = %+v", body)
	}
}

func TestMarkReadViewReturnsSuccessMessage(t *testing.T) {
	t.Parallel()

	repo := &stubRepository{available: true, matched: true}

	recorder, ctx := newTestContext(http.MethodPut, "/v1/common/chat/read")
	ctx.Set("uid", "7")
	ctx.Set("request", MarkReadReq{PeerID: 3})

	newHandler(repo, &stubQuery{}).MarkReadView(ctx)

	body := decodeResponse(t, recorder)
	require.EqualValues(t, res.SuccessCode, body.Code, "body = %+v", body)
	require.Equal(t, "标记成功", body.Msg, "body = %+v", body)

	require.EqualValues(t, 7, repo.readFor, "marked read for (%d, %d), want (7, 3)", repo.readFor, repo.readPeer)
	require.EqualValues(t, 3, repo.readPeer, "marked read for (%d, %d), want (7, 3)", repo.readFor, repo.readPeer)
}

// TestMarkReadViewRejectsSelfPeer 已读路径的自发文案是「peerId 无效」。
func TestMarkReadViewRejectsSelfPeer(t *testing.T) {
	t.Parallel()

	recorder, ctx := newTestContext(http.MethodPut, "/v1/common/chat/read")
	ctx.Set("uid", "7")
	ctx.Set("request", MarkReadReq{PeerID: 7})

	newHandler(&stubRepository{available: true}, &stubQuery{}).MarkReadView(ctx)

	{
		body := decodeResponse(t, recorder)
		require.Equal(t, body.Msg, domain.ErrInvalidPeerID.Error(), "body = %+v", body)
	}
}

// ---------------- WSView ----------------

// TestWSViewRequiresLogin WS 入口从 c.GetString("uid") 取身份，缺失或非法时返回「请先登录」，
// 不会走到 hub（hub 为 nil 也不 panic）。
func TestWSViewRequiresLogin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		uid  any
		set  bool
	}{
		{name: "missing uid", set: false},
		{name: "empty uid", uid: "", set: true},
		{name: "non numeric uid", uid: "abc", set: true},
		{name: "zero uid", uid: "0", set: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder, ctx := newTestContext(http.MethodGet, "/v1/common/chat/ws")
			if tt.set {
				ctx.Set("uid", tt.uid)
			}

			newHandler(&stubRepository{available: true}, &stubQuery{}).WSView(ctx)

			{
				body := decodeResponse(t, recorder)
				require.Equal(t, testLoginRequired, body.Msg, "body = %+v", body)
			}
		})
	}
}

// ---------------- stubs ----------------

// stubRepository 实现 domain.Repository（只覆盖处理器用例走到的分支）。
type stubRepository struct {
	available bool

	conversation *domain.Conversation
	matched      bool

	hiddenFor  uint
	hiddenPeer uint
	readFor    uint
	readPeer   uint
}

func (s *stubRepository) Available() bool { return s.available }

func (s *stubRepository) EnsureIndexes(context.Context) error { return nil }

func (s *stubRepository) InsertMessage(_ context.Context, msg *domain.Message) error {
	msg.ID = "652f1a2b3c4d5e6f70819200"

	return nil
}

func (s *stubRepository) ApplySend(context.Context, *domain.Message, string) error { return nil }

func (s *stubRepository) SendMessage(ctx context.Context, msg *domain.Message, preview string) error {
	if err := s.InsertMessage(ctx, msg); err != nil {
		return err
	}

	return s.ApplySend(ctx, msg, preview)
}

func (s *stubRepository) UpsertConversation(
	context.Context,
	uint, uint,
	time.Time,
) (*domain.Conversation, error) {
	if s.conversation == nil {
		return nil, domain.ErrConversationNotFound
	}

	return s.conversation, nil
}

func (s *stubRepository) HideConversation(_ context.Context, userID, peerID uint, _ time.Time) error {
	s.hiddenFor, s.hiddenPeer = userID, peerID

	return nil
}

func (s *stubRepository) MarkRead(_ context.Context, userID, peerID uint, _ time.Time) (bool, error) {
	s.readFor, s.readPeer = userID, peerID

	return s.matched, nil
}

func (s *stubRepository) ExistsMessageBetween(
	context.Context,
	string,
	uint, uint,
	time.Time, time.Time,
) (bool, error) {
	return false, nil
}

// stubQuery 实现 chatapp.Query。
type stubQuery struct {
	conversations []chatapp.ConversationItem
	total         int64
	messages      []chatapp.MessageItem

	lastPage     int
	lastPageSize int
}

func (s *stubQuery) ListConversations(
	_ context.Context,
	_ uint,
	page, pageSize int,
) ([]chatapp.ConversationItem, int64, error) {
	s.lastPage, s.lastPageSize = page, pageSize

	return s.conversations, s.total, nil
}

func (s *stubQuery) TotalUnread(context.Context, uint) (int64, error) { return 0, nil }

func (s *stubQuery) ListMessagesBefore(
	context.Context,
	uint, uint,
	string,
	int,
) ([]chatapp.MessageItem, error) {
	return s.messages, nil
}

// stubFollows 实现 chatapp.FollowRelationQuery。
type stubFollows struct {
	following bool
}

func (s *stubFollows) IsFollowing(context.Context, uint, uint) (bool, error) {
	return s.following, nil
}
