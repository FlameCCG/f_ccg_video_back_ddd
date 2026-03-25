package chat

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBuildConversationIDAndSlot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		a        uint
		b        uint
		wantID   string
		wantSlot MemberSlot
	}{
		{name: "ascending", a: 3, b: 9, wantID: "3_9", wantSlot: MemberA},
		{name: "descending", a: 9, b: 3, wantID: "3_9", wantSlot: MemberB},
		{name: "equal", a: 7, b: 7, wantID: "7_7", wantSlot: MemberA},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var conv Conversation
			if got := conv.BuildConversationID(tt.a, tt.b); got != tt.wantID {
				t.Fatalf("BuildConversationID(%d, %d) = %q, want %q", tt.a, tt.b, got, tt.wantID)
			}

			if got := conv.SlotOf(tt.a, tt.b); got != tt.wantSlot {
				t.Fatalf("SlotOf(%d, %d) = %q, want %q", tt.a, tt.b, got, tt.wantSlot)
			}
		})
	}
}

func TestValidateSender(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		senderID   uint
		receiverID uint
		wantErr    error
	}{
		{name: "ok", senderID: 1, receiverID: 2, wantErr: nil},
		{name: "no sender", senderID: 0, receiverID: 2, wantErr: ErrNotLoggedIn},
		{name: "no receiver", senderID: 1, receiverID: 0, wantErr: ErrNotLoggedIn},
		{name: "self", senderID: 5, receiverID: 5, wantErr: ErrSelfMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := (&Message{senderID: tt.senderID, receiverID: tt.receiverID}).EnsureParticipants(); !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateSender(%d, %d) = %v, want %v", tt.senderID, tt.receiverID, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePeer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		userID  uint
		peerID  uint
		selfErr error
		wantErr error
	}{
		{name: "ok", userID: 1, peerID: 2, selfErr: ErrInvalidPeerID, wantErr: nil},
		{name: "not logged in", userID: 0, peerID: 2, selfErr: ErrInvalidPeerID, wantErr: ErrNotLoggedIn},
		{name: "empty peer", userID: 1, peerID: 0, selfErr: ErrInvalidPeerID, wantErr: ErrEmptyPeerID},
		{name: "self on read path", userID: 3, peerID: 3, selfErr: ErrInvalidPeerID, wantErr: ErrInvalidPeerID},
		{name: "self on create path", userID: 3, peerID: 3, selfErr: ErrSelfConversation, wantErr: ErrSelfConversation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := (*RestoreConversation(ConversationSnapshot{})).ValidatePeer(tt.userID, tt.peerID, tt.selfErr); !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidatePeer(%d, %d) = %v, want %v", tt.userID, tt.peerID, err, tt.wantErr)
			}
		})
	}
}

func TestValidateContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		msgType string
		text    string
		emoji   string
		media   *Media
		wantErr error
	}{
		{name: "text ok", msgType: MessageTypeText, text: "hi", wantErr: nil},
		{name: "text empty", msgType: MessageTypeText, wantErr: ErrEmptyText},
		{name: "text too long", msgType: MessageTypeText, text: strings.Repeat("字", 1001), wantErr: ErrTextTooLong},
		{name: "text at limit", msgType: MessageTypeText, text: strings.Repeat("字", 1000), wantErr: nil},
		{name: "emoji ok", msgType: MessageTypeEmoji, emoji: ":)", wantErr: nil},
		{name: "emoji falls back to text", msgType: MessageTypeEmoji, text: ":)", wantErr: nil},
		{name: "emoji empty", msgType: MessageTypeEmoji, wantErr: ErrEmptyEmoji},
		{name: "image ok", msgType: MessageTypeImage, media: &Media{URL: "http://x/a.png"}, wantErr: nil},
		{name: "image nil media", msgType: MessageTypeImage, wantErr: ErrEmptyMediaURL},
		{name: "sticker empty url", msgType: MessageTypeSticker, media: &Media{}, wantErr: ErrEmptyMediaURL},
		{name: "unknown type", msgType: "video", wantErr: ErrUnsupportedMessageType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := (&Message{messageType: tt.msgType, text: tt.text, emoji: tt.emoji, media: tt.media}).ValidateContent(); !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateContent(%q) = %v, want %v", tt.msgType, err, tt.wantErr)
			}
		})
	}
}

func TestBuildPreview(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  *Message
		want string
	}{
		{name: "nil", msg: nil, want: ""},
		{name: "text short", msg: &Message{messageType: MessageTypeText, text: "hello"}, want: "hello"},
		{name: "text empty", msg: &Message{messageType: MessageTypeText}, want: ""},
		{
			name: "text truncated",
			msg:  &Message{messageType: MessageTypeText, text: strings.Repeat("字", 51)},
			want: strings.Repeat("字", 50) + "...",
		},
		{name: "image", msg: &Message{messageType: MessageTypeImage}, want: "[图片]"},
		{name: "emoji with value", msg: &Message{messageType: MessageTypeEmoji, emoji: ":)"}, want: ":)"},
		{name: "emoji without value", msg: &Message{messageType: MessageTypeEmoji}, want: "[表情]"},
		{name: "sticker", msg: &Message{messageType: MessageTypeSticker}, want: "[表情包]"},
		{name: "unknown", msg: &Message{messageType: "audio"}, want: "[消息]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.msg.Preview(); got != tt.want {
				t.Fatalf("BuildPreview = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEvaluateDailySendLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		sentToday    bool
		repliedToday bool
		wantErr      error
	}{
		{name: "nothing sent today", sentToday: false, repliedToday: false, wantErr: nil},
		{name: "sent and replied", sentToday: true, repliedToday: true, wantErr: nil},
		{name: "sent without reply", sentToday: true, repliedToday: false, wantErr: ErrDailySendLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := (SendLimit{}).EvaluateDailySendLimit(DailySendFacts{SentToday: tt.sentToday, RepliedToday: tt.repliedToday}); !errors.Is(err, tt.wantErr) {
				t.Fatalf("EvaluateDailySendLimit(%v, %v) = %v, want %v",
					tt.sentToday, tt.repliedToday, err, tt.wantErr)
			}
		})
	}
}

func TestNeedsDailySendLimitCheck(t *testing.T) {
	t.Parallel()

	var limit SendLimit
	if limit.NeedsDailySendLimitCheck(0, 2) || limit.NeedsDailySendLimitCheck(1, 0) || limit.NeedsDailySendLimitCheck(4, 4) {
		t.Fatal("NeedsDailySendLimitCheck should skip incomplete or self pairs")
	}

	if !limit.NeedsDailySendLimitCheck(1, 2) {
		t.Fatal("NeedsDailySendLimitCheck(1, 2) should require the check")
	}
}

func TestDayRange(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 26, 13, 45, 30, 500, time.FixedZone("CST", 8*3600))

	start, end := (SendLimit{}).DayRange(now)
	if !start.Equal(time.Date(2026, 7, 26, 0, 0, 0, 0, now.Location())) {
		t.Fatalf("DayRange start = %v", start)
	}

	if !end.Equal(time.Date(2026, 7, 27, 0, 0, 0, 0, now.Location())) {
		t.Fatalf("DayRange end = %v", end)
	}
}

func TestConversationPeerAndUnread(t *testing.T) {
	t.Parallel()

	conv := &Conversation{
		memberA: Member{UserID: 3, Unread: 1},
		memberB: Member{UserID: 9, Unread: 4},
	}

	if peer, unread := conv.PeerAndUnread(3); peer != 9 || unread != 1 {
		t.Fatalf("PeerAndUnread(3) = (%d, %d), want (9, 1)", peer, unread)
	}

	if peer, unread := conv.PeerAndUnread(9); peer != 3 || unread != 4 {
		t.Fatalf("PeerAndUnread(9) = (%d, %d), want (3, 4)", peer, unread)
	}

	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	conv.createdAt = created

	if got := conv.DisplayTime(); !got.Equal(created) {
		t.Fatalf("DisplayTime with zero UpdatedAt = %v, want %v", got, created)
	}

	updated := created.Add(time.Hour)
	conv.updatedAt = updated

	if got := conv.DisplayTime(); !got.Equal(updated) {
		t.Fatalf("DisplayTime = %v, want %v", got, updated)
	}
}

func TestNewMessagePublishesParticipantsWithConversationID(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	msg, err := NewMessage(9, 3, MessageTypeText, "一起", "", nil, now)
	if err != nil {
		t.Fatal(err)
	}

	if msg.SenderID() != 9 || msg.ReceiverID() != 3 || msg.ConversationID() != (*RestoreConversation(ConversationSnapshot{})).BuildConversationID(9, 3) {
		t.Fatalf("related identity diverged: %+v", msg.Snapshot())
	}

	if !msg.CreatedAt().Equal(now) || msg.Text() != "一起" || msg.Type() != MessageTypeText || msg.Preview() != "一起" {
		t.Fatalf("content diverged from identity: %+v", msg.Snapshot())
	}

	partial, err := NewMessage(9, 9, MessageTypeText, "一起", "", nil, now)
	if partial != nil || !errors.Is(err, ErrSelfMessage) {
		t.Fatalf("failed create leaked state: msg=%+v err=%v", partial, err)
	}
}

func TestValidateFailureLeavesRestoredMessageUnchanged(t *testing.T) {
	t.Parallel()

	created := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
	media := &Media{URL: "keep.png", Width: 3}
	msg := RestoreMessage(MessageSnapshot{
		ID:             "kept",
		ConversationID: "1_1",
		SenderID:       1,
		ReceiverID:     1,
		Type:           MessageTypeText,
		Text:           "",
		Media:          media,
		CreatedAt:      created,
	})
	before := msg.Snapshot()

	if err := msg.Validate(); !errors.Is(err, ErrSelfMessage) {
		t.Fatalf("err = %v, want participant error before content", err)
	}

	after := msg.Snapshot()
	if before.ID != after.ID || before.ConversationID != after.ConversationID || before.SenderID != after.SenderID || before.ReceiverID != after.ReceiverID || before.Type != after.Type || before.Text != after.Text || !before.CreatedAt.Equal(after.CreatedAt) {
		t.Fatalf("failed validation changed message: before=%+v after=%+v", before, after)
	}

	if after.Media == nil || after.Media.URL != "keep.png" || after.Media.Width != 3 {
		t.Fatalf("media = %+v", after.Media)
	}

	media.URL = "caller.png"
	media.Width = 9
	if msg.Media().URL != "keep.png" || msg.Media().Width != 3 {
		t.Fatal("restore aliased caller media")
	}

	msg.Media().URL = "getter.png"
	if msg.Media().URL != "keep.png" {
		t.Fatal("Media() aliased internal media")
	}

	snap := msg.Snapshot()
	snap.Media.URL = "snap.png"
	snap.Text = "changed"
	if msg.Text() != "" || msg.Media().URL != "keep.png" || msg.ID() != "kept" {
		t.Fatal("snapshot aliased the message")
	}
}

func TestConversationSnapshotCopiesUnreadAndPreview(t *testing.T) {
	t.Parallel()

	created := time.Unix(1, 0).UTC()
	updated := time.Unix(2, 0).UTC()
	last := &LastMessage{MsgID: "m1", Type: MessageTypeText, Preview: "hi", SenderID: 3, CreatedAt: created}
	conv := RestoreConversation(ConversationSnapshot{
		ID:          "3_9",
		MemberA:     Member{UserID: 3, Unread: 1, Visible: true},
		MemberB:     Member{UserID: 9, Unread: 4, Visible: true},
		LastMessage: last,
		CreatedAt:   created,
		UpdatedAt:   updated,
	})

	last.Preview = "nope"
	last.SenderID = 9
	if conv.LastMessage().Preview != "hi" || conv.LastMessage().SenderID != 3 {
		t.Fatal("restore aliased last message")
	}

	view := conv.LastMessage()
	view.Preview = "changed"
	view.SenderID = 1
	if conv.LastMessage().Preview != "hi" || conv.LastMessage().SenderID != 3 {
		t.Fatal("LastMessage() aliased internal summary")
	}

	member := conv.MemberA()
	member.Unread = 100
	member.Visible = false
	peer, unread := conv.PeerAndUnread(3)
	if peer != 9 || unread != 1 || !conv.MemberA().Visible {
		t.Fatalf("member copy changed conversation: peer=%d unread=%d visible=%v", peer, unread, conv.MemberA().Visible)
	}

	snap := conv.Snapshot()
	snap.MemberB.Unread = 0
	snap.LastMessage.Preview = "snap"
	snap.ID = "other"
	if conv.ID() != "3_9" || conv.LastMessage().Preview != "hi" {
		t.Fatal("snapshot aliased conversation")
	}

	if _, unread := conv.PeerAndUnread(9); unread != 4 {
		t.Fatalf("snapshot changed unread to %d", unread)
	}

	if !conv.DisplayTime().Equal(updated) || !conv.CreatedAt().Equal(created) {
		t.Fatal("timestamps diverged")
	}
}
