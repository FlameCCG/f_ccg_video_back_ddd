package chat

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/chat"

	"go.uber.org/zap"
)

// 对应原 service/chat_service（原文件仅有 page_test.go，其余分支此处补齐）。
//
// 重点锁定两类契约：
//  1. Mongo 未初始化时每个方法都返回「mongo 未初始化」（原 getChatDB 的降级行为）；
//  2. 单日发送限制的编排（关注关系查询失败只告警放行）。

// newService 造一个 Mongo 可用、对端未关注自己、今日无消息的服务。
func newService() (*Service, *fakeRepository, *fakeQuery, *fakeFollowRelation, *fakeBadgeInvalidator) {
	repo := &fakeRepository{available: true, exists: make(map[messageWindow]bool)}
	query := &fakeQuery{}
	follows := &fakeFollowRelation{following: true}
	badges := &fakeBadgeInvalidator{}

	return NewService(repo, query, follows, badges, zap.NewNop()), repo, query, follows, badges
}

// TestAllMethodsRequireStore Mongo 缺失时全部用例方法返回 ErrStoreUnavailable，
// 且不触碰读侧查询。
func TestAllMethodsRequireStore(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{available: false}
	query := &fakeQuery{}
	svc := NewService(repo, query, &fakeFollowRelation{}, &fakeBadgeInvalidator{}, zap.NewNop())
	ctx := context.Background()

	if _, err := svc.SendMessage(ctx, 1, 2, domain.MessageTypeText, "hi", "", nil); !errors.Is(err, domain.ErrStoreUnavailable) {
		t.Fatalf("SendMessage err = %v", err)
	}

	if _, _, err := svc.ListConversations(ctx, 1, 1, 20); !errors.Is(err, domain.ErrStoreUnavailable) {
		t.Fatalf("ListConversations err = %v", err)
	}

	if _, err := svc.ListMessages(ctx, 1, 2, ""); !errors.Is(err, domain.ErrStoreUnavailable) {
		t.Fatalf("ListMessages err = %v", err)
	}

	if err := svc.MarkRead(ctx, 1, 2); !errors.Is(err, domain.ErrStoreUnavailable) {
		t.Fatalf("MarkRead err = %v", err)
	}

	if _, err := svc.TotalUnread(ctx, 1); !errors.Is(err, domain.ErrStoreUnavailable) {
		t.Fatalf("TotalUnread err = %v", err)
	}

	if _, err := svc.CreateConversation(ctx, 1, 2); !errors.Is(err, domain.ErrStoreUnavailable) {
		t.Fatalf("CreateConversation err = %v", err)
	}

	if err := svc.DeleteConversation(ctx, 1, 2); !errors.Is(err, domain.ErrStoreUnavailable) {
		t.Fatalf("DeleteConversation err = %v", err)
	}

	if query.lastPageSize != 0 || query.lastLimit != 0 {
		t.Fatal("read side must not be queried when the store is unavailable")
	}
}

// TestSendMessageValidatesParticipantsAndContent 参与者与内容校验的错误文案顺序
// 与原 SendMessage 完全一致（文案会原样返回给前端）。
func TestSendMessageValidatesParticipantsAndContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		senderID   uint
		receiverID uint
		msgType    string
		text       string
		emoji      string
		media      *MediaItem
		wantErr    error
	}{
		{name: "no sender", senderID: 0, receiverID: 2, msgType: domain.MessageTypeText, text: "hi", wantErr: domain.ErrNotLoggedIn},
		{name: "no receiver", senderID: 1, receiverID: 0, msgType: domain.MessageTypeText, text: "hi", wantErr: domain.ErrNotLoggedIn},
		{name: "self", senderID: 1, receiverID: 1, msgType: domain.MessageTypeText, text: "hi", wantErr: domain.ErrSelfMessage},
		{name: "empty text", senderID: 1, receiverID: 2, msgType: domain.MessageTypeText, wantErr: domain.ErrEmptyText},
		{
			name:     "text too long",
			senderID: 1, receiverID: 2,
			msgType: domain.MessageTypeText,
			text:    strings.Repeat("字", 1001),
			wantErr: domain.ErrTextTooLong,
		},
		{name: "empty emoji", senderID: 1, receiverID: 2, msgType: domain.MessageTypeEmoji, wantErr: domain.ErrEmptyEmoji},
		{name: "image without media", senderID: 1, receiverID: 2, msgType: domain.MessageTypeImage, wantErr: domain.ErrEmptyMediaURL},
		{
			name:     "sticker with blank url",
			senderID: 1, receiverID: 2,
			msgType: domain.MessageTypeSticker,
			media:   &MediaItem{},
			wantErr: domain.ErrEmptyMediaURL,
		},
		{name: "unsupported type", senderID: 1, receiverID: 2, msgType: "audio", wantErr: domain.ErrUnsupportedMessageType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, repo, _, _, _ := newService()

			_, err := svc.SendMessage(context.Background(), tt.senderID, tt.receiverID, tt.msgType, tt.text, tt.emoji, tt.media)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}

			if len(repo.inserted) != 0 {
				t.Fatal("invalid message must not be persisted")
			}
		})
	}
}

// TestSendMessagePersistsAndAdvancesConversation 落库 → 推进会话摘要 → 失效接收方角标。
func TestSendMessagePersistsAndAdvancesConversation(t *testing.T) {
	t.Parallel()

	svc, repo, _, _, badges := newService()

	msg, err := svc.SendMessage(context.Background(), 9, 3, domain.MessageTypeText, "你好", "", nil)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if msg.ID != repo.insertedID {
		t.Fatalf("returned id = %q, want %q", msg.ID, repo.insertedID)
	}

	// conversationId 是确定性的「小ID_大ID」。
	if msg.ConversationID != "3_9" {
		t.Fatalf("conversationId = %q, want 3_9", msg.ConversationID)
	}

	if msg.SenderID != 9 || msg.ReceiverID != 3 || msg.Type != domain.MessageTypeText || msg.Text != "你好" {
		t.Fatalf("message = %+v", msg)
	}

	if msg.CreatedAt.IsZero() {
		t.Fatal("createdAt must be set")
	}

	if len(repo.applied) != 1 || repo.previews[0] != "你好" {
		t.Fatalf("applied = %+v previews = %v", repo.applied, repo.previews)
	}

	// 角标失效针对接收方。
	if !reflect.DeepEqual(badges.invalidated, []int{3}) {
		t.Fatalf("invalidated = %v, want [3]", badges.invalidated)
	}
}

// TestSendMessageBuildsPreviewPerType 会话摘要预览按消息类型生成。
func TestSendMessageBuildsPreviewPerType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		msgType string
		text    string
		emoji   string
		media   *MediaItem
		want    string
	}{
		{name: "text", msgType: domain.MessageTypeText, text: "简短", want: "简短"},
		{
			name:    "long text truncated",
			msgType: domain.MessageTypeText,
			text:    strings.Repeat("字", 51),
			want:    strings.Repeat("字", 50) + "...",
		},
		{name: "image", msgType: domain.MessageTypeImage, media: &MediaItem{URL: "http://x/a.png"}, want: "[图片]"},
		{name: "emoji with value", msgType: domain.MessageTypeEmoji, emoji: ":)", want: ":)"},
		{name: "emoji from text", msgType: domain.MessageTypeEmoji, text: "^_^", want: "[表情]"},
		{name: "sticker", msgType: domain.MessageTypeSticker, media: &MediaItem{URL: "http://x/s.png"}, want: "[表情包]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, repo, _, _, _ := newService()

			if _, err := svc.SendMessage(
				context.Background(), 1, 2, tt.msgType, tt.text, tt.emoji, tt.media,
			); err != nil {
				t.Fatalf("SendMessage: %v", err)
			}

			if len(repo.previews) != 1 || repo.previews[0] != tt.want {
				t.Fatalf("preview = %v, want %q", repo.previews, tt.want)
			}
		})
	}
}

// TestSendMessageCarriesMediaThroughDomain 媒体字段在 DTO ↔ 领域 ↔ 响应之间往返不丢字段。
func TestSendMessageCarriesMediaThroughDomain(t *testing.T) {
	t.Parallel()

	svc, repo, _, _, _ := newService()
	media := &MediaItem{URL: "http://x/a.png", Mime: "image/png", Width: 4, Height: 5, Size: 678}

	msg, err := svc.SendMessage(context.Background(), 1, 2, domain.MessageTypeImage, "", "", media)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if msg.Media == nil || *msg.Media != *media {
		t.Fatalf("media = %+v, want %+v", msg.Media, media)
	}

	if got := repo.inserted[0].Media; got == nil || got.URL != media.URL || got.Size != media.Size {
		t.Fatalf("persisted media = %+v", got)
	}
}

// TestSendMessageSkipsLimitWhenReceiverFollowsSender 对端已关注自己时不做频率判定。
func TestSendMessageSkipsLimitWhenReceiverFollowsSender(t *testing.T) {
	t.Parallel()

	svc, repo, _, follows, _ := newService()
	follows.following = true

	if _, err := svc.SendMessage(context.Background(), 1, 2, domain.MessageTypeText, "hi", "", nil); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	// 关注关系查询的方向是「接收方是否关注发送方」。
	if !reflect.DeepEqual(follows.calls, []messageWindow{{senderID: 2, receiverID: 1}}) {
		t.Fatalf("follow calls = %+v", follows.calls)
	}

	if len(repo.windows) != 0 {
		t.Fatalf("follower path must skip the message window lookup: %+v", repo.windows)
	}
}

// TestSendMessageEnforcesDailyLimitForNonFollower 对端未关注自己且今日已发过一条、
// 对端未回复时拒绝发送；文案属于外部契约。
func TestSendMessageEnforcesDailyLimitForNonFollower(t *testing.T) {
	t.Parallel()

	svc, repo, _, follows, _ := newService()
	follows.following = false
	repo.exists[messageWindow{conversationID: "1_2", senderID: 1, receiverID: 2}] = true

	_, err := svc.SendMessage(context.Background(), 1, 2, domain.MessageTypeText, "hi", "", nil)
	if !errors.Is(err, domain.ErrDailySendLimit) {
		t.Fatalf("err = %v, want ErrDailySendLimit", err)
	}

	if len(repo.inserted) != 0 {
		t.Fatal("rejected message must not be persisted")
	}
}

// TestSendMessageAllowsSecondMessageAfterReply 对端今日回复过就解除限制。
func TestSendMessageAllowsSecondMessageAfterReply(t *testing.T) {
	t.Parallel()

	svc, repo, _, follows, _ := newService()
	follows.following = false
	repo.exists[messageWindow{conversationID: "1_2", senderID: 1, receiverID: 2}] = true
	repo.exists[messageWindow{conversationID: "1_2", senderID: 2, receiverID: 1}] = true

	if _, err := svc.SendMessage(context.Background(), 1, 2, domain.MessageTypeText, "hi", "", nil); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if len(repo.inserted) != 1 {
		t.Fatalf("inserted = %+v", repo.inserted)
	}
}

// TestSendMessageAllowsFirstMessageOfDay 今日尚未发过消息时直接放行（不查回复方向）。
func TestSendMessageAllowsFirstMessageOfDay(t *testing.T) {
	t.Parallel()

	svc, repo, _, follows, _ := newService()
	follows.following = false

	if _, err := svc.SendMessage(context.Background(), 1, 2, domain.MessageTypeText, "hi", "", nil); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if len(repo.windows) != 1 {
		t.Fatalf("windows = %+v, want only the outbound lookup", repo.windows)
	}
}

// TestSendMessageIgnoresFollowLookupFailure 关注关系查询失败只告警并放行，
// 不因基础设施抖动阻断用户发消息（与原实现一致）。
func TestSendMessageIgnoresFollowLookupFailure(t *testing.T) {
	t.Parallel()

	svc, repo, _, follows, _ := newService()
	follows.err = errors.New("db down")

	if _, err := svc.SendMessage(context.Background(), 1, 2, domain.MessageTypeText, "hi", "", nil); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if len(repo.inserted) != 1 {
		t.Fatalf("inserted = %+v", repo.inserted)
	}

	if len(repo.windows) != 0 {
		t.Fatal("failed follow lookup must skip the rate-limit windows")
	}
}

// TestSendMessagePropagatesWindowLookupError 消息窗口查询报错要向上传播（区别于关注关系查询）。
func TestSendMessagePropagatesWindowLookupError(t *testing.T) {
	t.Parallel()

	svc, repo, _, follows, _ := newService()
	follows.following = false
	repo.existsErr = errors.New("mongo down")

	wantErr := repo.existsErr
	if _, err := svc.SendMessage(context.Background(), 1, 2, domain.MessageTypeText, "hi", "", nil); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func TestSendMessagePropagatesPersistenceErrors(t *testing.T) {
	t.Parallel()

	t.Run("insert", func(t *testing.T) {
		t.Parallel()

		svc, repo, _, _, badges := newService()
		repo.insertErr = errors.New("insert failed")

		if _, err := svc.SendMessage(
			context.Background(), 1, 2, domain.MessageTypeText, "hi", "", nil,
		); !errors.Is(err, repo.insertErr) {
			t.Fatalf("err = %v", err)
		}

		if len(badges.invalidated) != 0 {
			t.Fatal("failed send must not invalidate the badge cache")
		}
	})

	t.Run("apply", func(t *testing.T) {
		t.Parallel()

		svc, repo, _, _, badges := newService()
		repo.applyErr = errors.New("upsert failed")

		if _, err := svc.SendMessage(
			context.Background(), 1, 2, domain.MessageTypeText, "hi", "", nil,
		); !errors.Is(err, repo.applyErr) {
			t.Fatalf("err = %v", err)
		}

		if len(badges.invalidated) != 0 {
			t.Fatal("failed conversation upsert must not invalidate the badge cache")
		}
	})
}

// TestSendMessageToleratesMissingBadgePort 角标端口未绑定时静默跳过，不 panic。
func TestSendMessageToleratesMissingBadgePort(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{available: true, exists: make(map[messageWindow]bool)}
	svc := NewService(repo, &fakeQuery{}, &fakeFollowRelation{following: true}, nil, zap.NewNop())

	if _, err := svc.SendMessage(context.Background(), 1, 2, domain.MessageTypeText, "hi", "", nil); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
}

// TestListConversationsNormalizesPaging 分页参数由领域规则二次 clamp（默认 20、上限 50）。
func TestListConversationsNormalizesPaging(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		page         int
		pageSize     int
		wantPage     int
		wantPageSize int
	}{
		{name: "defaults", page: 0, pageSize: 0, wantPage: 1, wantPageSize: domain.DefaultConversationPageSize},
		{name: "clamped", page: 2, pageSize: 1_000, wantPage: 2, wantPageSize: domain.MaxConversationPageSize},
		{name: "kept", page: 3, pageSize: 15, wantPage: 3, wantPageSize: 15},
		{name: "negative", page: -1, pageSize: -5, wantPage: 1, wantPageSize: domain.DefaultConversationPageSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, _, query, _, _ := newService()
			query.total = 7

			_, total, err := svc.ListConversations(context.Background(), 1, tt.page, tt.pageSize)
			if err != nil {
				t.Fatalf("ListConversations: %v", err)
			}

			if total != 7 {
				t.Fatalf("total = %d, want 7", total)
			}

			if query.lastPage != tt.wantPage || query.lastPageSize != tt.wantPageSize {
				t.Fatalf("paging = (%d, %d), want (%d, %d)",
					query.lastPage, query.lastPageSize, tt.wantPage, tt.wantPageSize)
			}
		})
	}
}

func TestListConversationsRejectsAnonymous(t *testing.T) {
	t.Parallel()

	svc, _, query, _, _ := newService()

	if _, _, err := svc.ListConversations(context.Background(), 0, 1, 20); !errors.Is(err, domain.ErrNotLoggedIn) {
		t.Fatalf("err = %v, want ErrNotLoggedIn", err)
	}

	if query.lastPageSize != 0 {
		t.Fatal("anonymous request must not reach the read side")
	}
}

// TestListMessagesValidatesPeerAndCursor 参与者与游标校验；游标必须是 24 位十六进制。
func TestListMessagesValidatesPeerAndCursor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		userID  uint
		peerID  uint
		before  string
		wantErr error
	}{
		{name: "not logged in", userID: 0, peerID: 2, wantErr: domain.ErrNotLoggedIn},
		{name: "empty peer", userID: 1, peerID: 0, wantErr: domain.ErrEmptyPeerID},
		{name: "self peer", userID: 1, peerID: 1, wantErr: domain.ErrInvalidPeerID},
		{name: "bad cursor", userID: 1, peerID: 2, before: "zzz", wantErr: domain.ErrInvalidCursor},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, _, query, _, _ := newService()

			if _, err := svc.ListMessages(context.Background(), tt.userID, tt.peerID, tt.before); !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}

			if query.lastLimit != 0 {
				t.Fatal("invalid request must not reach the read side")
			}
		})
	}
}

// TestListMessagesTrimsPageAndSetsCursor 多取一条判断 hasMore，结果倒序（旧→新），
// nextBefore 取该页最早一条的 ID。
func TestListMessagesTrimsPageAndSetsCursor(t *testing.T) {
	t.Parallel()

	// 仓储按 _id 降序返回 11 条（MessageFetchLimit）。
	rows := make([]MessageItem, 0, domain.MessageFetchLimit)
	for i := domain.MessageFetchLimit; i >= 1; i-- {
		rows = append(rows, MessageItem{ID: strconv.Itoa(i)})
	}

	svc, _, query, _, _ := newService()
	query.messages = rows

	res, err := svc.ListMessages(context.Background(), 1, 2, "")
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}

	if query.lastLimit != domain.MessageFetchLimit {
		t.Fatalf("limit = %d, want %d", query.lastLimit, domain.MessageFetchLimit)
	}

	if !res.HasMore {
		t.Fatal("hasMore must be true when the extra row comes back")
	}

	if len(res.List) != domain.MessagePageSize {
		t.Fatalf("page size = %d, want %d", len(res.List), domain.MessagePageSize)
	}

	// 截掉第 11 条后倒序：最早的是 "2"，最新的是 "11"。
	if res.List[0].ID != "2" || res.List[len(res.List)-1].ID != "11" {
		t.Fatalf("page = %+v", res.List)
	}

	if res.NextBefore != "2" {
		t.Fatalf("nextBefore = %q, want 2", res.NextBefore)
	}
}

// TestListMessagesWithoutMoreRowsHasNoCursor 不足一页时 hasMore=false、不带游标。
func TestListMessagesWithoutMoreRowsHasNoCursor(t *testing.T) {
	t.Parallel()

	svc, _, query, _, _ := newService()
	query.messages = []MessageItem{{ID: "3"}, {ID: "2"}, {ID: "1"}}

	res, err := svc.ListMessages(context.Background(), 1, 2, "")
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}

	if res.HasMore || res.NextBefore != "" {
		t.Fatalf("res = %+v", res)
	}

	if got := []string{res.List[0].ID, res.List[1].ID, res.List[2].ID}; !reflect.DeepEqual(got, []string{"1", "2", "3"}) {
		t.Fatalf("order = %v, want [1 2 3]", got)
	}
}

// TestListMessagesForwardsValidCursor 合法游标原样传给读侧。
func TestListMessagesForwardsValidCursor(t *testing.T) {
	t.Parallel()

	svc, _, query, _, _ := newService()
	cursor := "652f1a2b3c4d5e6f70819200"

	if _, err := svc.ListMessages(context.Background(), 1, 2, cursor); err != nil {
		t.Fatalf("ListMessages: %v", err)
	}

	if query.lastBefore != cursor {
		t.Fatalf("before = %q, want %q", query.lastBefore, cursor)
	}
}

// TestMarkReadInvalidatesBadgeOnlyWhenMatched 未命中会话时不联动角标（原实现同样早返回）。
func TestMarkReadInvalidatesBadgeOnlyWhenMatched(t *testing.T) {
	t.Parallel()

	t.Run("matched", func(t *testing.T) {
		t.Parallel()

		svc, repo, _, _, badges := newService()
		repo.matched = true

		if err := svc.MarkRead(context.Background(), 7, 3); err != nil {
			t.Fatalf("MarkRead: %v", err)
		}

		// 已读清零的是当前用户自己的未读，因此角标失效针对本人。
		if !reflect.DeepEqual(badges.invalidated, []int{7}) {
			t.Fatalf("invalidated = %v, want [7]", badges.invalidated)
		}
	})

	t.Run("not matched", func(t *testing.T) {
		t.Parallel()

		svc, repo, _, _, badges := newService()
		repo.matched = false

		if err := svc.MarkRead(context.Background(), 7, 3); err != nil {
			t.Fatalf("MarkRead: %v", err)
		}

		if len(badges.invalidated) != 0 {
			t.Fatalf("invalidated = %v, want none", badges.invalidated)
		}
	})
}

func TestMarkReadValidatesPeer(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _ := newService()
	ctx := context.Background()

	if err := svc.MarkRead(ctx, 0, 2); !errors.Is(err, domain.ErrNotLoggedIn) {
		t.Fatalf("err = %v", err)
	}

	if err := svc.MarkRead(ctx, 1, 0); !errors.Is(err, domain.ErrEmptyPeerID) {
		t.Fatalf("err = %v", err)
	}

	if err := svc.MarkRead(ctx, 1, 1); !errors.Is(err, domain.ErrInvalidPeerID) {
		t.Fatalf("err = %v", err)
	}
}

func TestMarkReadPropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	svc, repo, _, _, _ := newService()
	repo.markErr = errors.New("mongo down")

	if err := svc.MarkRead(context.Background(), 1, 2); !errors.Is(err, repo.markErr) {
		t.Fatalf("err = %v", err)
	}
}

// TestTotalUnreadSatisfiesNotificationPort TotalUnread 是 notification 上下文
// ChatUnreadCounter 端口的实现，签名与语义都不能变。
func TestTotalUnreadSatisfiesNotificationPort(t *testing.T) {
	t.Parallel()

	svc, _, query, _, _ := newService()
	query.unread = 12

	total, err := svc.TotalUnread(context.Background(), 5)
	if err != nil {
		t.Fatalf("TotalUnread: %v", err)
	}

	if total != 12 {
		t.Fatalf("total = %d, want 12", total)
	}

	if _, err := svc.TotalUnread(context.Background(), 0); !errors.Is(err, domain.ErrNotLoggedIn) {
		t.Fatalf("anonymous err = %v, want ErrNotLoggedIn", err)
	}

	// 编译期确认端口签名匹配。
	var counter interface {
		TotalUnread(ctx context.Context, userID uint) (int64, error)
	} = svc
	_ = counter
}

// TestCreateConversationMapsPeerAndUnread 会话项按当前用户视角映射对端与未读数。
func TestCreateConversationMapsPeerAndUnread(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := created.Add(time.Hour)

	svc, repo, _, _, _ := newService()
	repo.upsert = &domain.Conversation{
		ID:          "3_9",
		MemberA:     domain.Member{UserID: 3, Unread: 1},
		MemberB:     domain.Member{UserID: 9, Unread: 4},
		LastMessage: &domain.LastMessage{MsgID: "abc", Type: domain.MessageTypeText, Preview: "hi", SenderID: 3, CreatedAt: created},
		CreatedAt:   created,
		UpdatedAt:   updated,
	}

	item, err := svc.CreateConversation(context.Background(), 9, 3)
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	if item.ID != "3_9" || item.PeerID != 3 || item.Unread != 4 {
		t.Fatalf("item = %+v", item)
	}

	if item.UpdatedAt != updated.Unix() {
		t.Fatalf("updatedAt = %d, want %d", item.UpdatedAt, updated.Unix())
	}

	if item.LastMessage == nil || item.LastMessage.MsgID != "abc" || item.LastMessage.Preview != "hi" {
		t.Fatalf("lastMessage = %+v", item.LastMessage)
	}
}

// TestCreateConversationFallsBackToCreatedAt updatedAt 为零值时回退 createdAt。
func TestCreateConversationFallsBackToCreatedAt(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)

	svc, repo, _, _, _ := newService()
	repo.upsert = &domain.Conversation{
		ID:        "1_2",
		MemberA:   domain.Member{UserID: 1},
		MemberB:   domain.Member{UserID: 2},
		CreatedAt: created,
	}

	item, err := svc.CreateConversation(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	if item.UpdatedAt != created.Unix() {
		t.Fatalf("updatedAt = %d, want %d", item.UpdatedAt, created.Unix())
	}

	// 空会话没有 lastMessage（响应中该字段被 omitempty 省略）。
	if item.LastMessage != nil {
		t.Fatalf("lastMessage = %+v, want nil", item.LastMessage)
	}
}

// TestCreateConversationUsesDedicatedSelfError 建会话路径的自发文案与其它路径不同。
func TestCreateConversationUsesDedicatedSelfError(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _ := newService()
	ctx := context.Background()

	if _, err := svc.CreateConversation(ctx, 5, 5); !errors.Is(err, domain.ErrSelfConversation) {
		t.Fatalf("err = %v, want ErrSelfConversation", err)
	}

	if _, err := svc.CreateConversation(ctx, 0, 2); !errors.Is(err, domain.ErrNotLoggedIn) {
		t.Fatalf("err = %v", err)
	}

	if _, err := svc.CreateConversation(ctx, 1, 0); !errors.Is(err, domain.ErrEmptyPeerID) {
		t.Fatalf("err = %v", err)
	}
}

func TestCreateConversationPropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	svc, repo, _, _, _ := newService()
	repo.upsertErr = domain.ErrConversationNotFound

	if _, err := svc.CreateConversation(context.Background(), 1, 2); !errors.Is(err, domain.ErrConversationNotFound) {
		t.Fatalf("err = %v, want ErrConversationNotFound", err)
	}
}

// TestDeleteConversationHidesForCurrentUserOnly 删除会话只对自己隐藏。
func TestDeleteConversationHidesForCurrentUserOnly(t *testing.T) {
	t.Parallel()

	svc, repo, _, _, _ := newService()

	if err := svc.DeleteConversation(context.Background(), 9, 3); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}

	if !reflect.DeepEqual(repo.hidden, []messageWindow{{senderID: 9, receiverID: 3}}) {
		t.Fatalf("hidden = %+v", repo.hidden)
	}
}

func TestDeleteConversationValidatesPeer(t *testing.T) {
	t.Parallel()

	svc, repo, _, _, _ := newService()
	ctx := context.Background()

	if err := svc.DeleteConversation(ctx, 0, 2); !errors.Is(err, domain.ErrNotLoggedIn) {
		t.Fatalf("err = %v", err)
	}

	if err := svc.DeleteConversation(ctx, 1, 0); !errors.Is(err, domain.ErrEmptyPeerID) {
		t.Fatalf("err = %v", err)
	}

	if err := svc.DeleteConversation(ctx, 1, 1); !errors.Is(err, domain.ErrInvalidPeerID) {
		t.Fatalf("err = %v", err)
	}

	if len(repo.hidden) != 0 {
		t.Fatal("invalid request must not reach the repository")
	}
}

func TestDeleteConversationPropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	svc, repo, _, _, _ := newService()
	repo.hideErr = errors.New("mongo down")

	if err := svc.DeleteConversation(context.Background(), 1, 2); !errors.Is(err, repo.hideErr) {
		t.Fatalf("err = %v", err)
	}
}
