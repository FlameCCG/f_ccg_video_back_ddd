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

			if got := BuildConversationID(tt.a, tt.b); got != tt.wantID {
				t.Fatalf("BuildConversationID(%d, %d) = %q, want %q", tt.a, tt.b, got, tt.wantID)
			}

			if got := SlotOf(tt.a, tt.b); got != tt.wantSlot {
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

			if err := ValidateSender(tt.senderID, tt.receiverID); !errors.Is(err, tt.wantErr) {
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

			if err := ValidatePeer(tt.userID, tt.peerID, tt.selfErr); !errors.Is(err, tt.wantErr) {
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

			if err := ValidateContent(tt.msgType, tt.text, tt.emoji, tt.media); !errors.Is(err, tt.wantErr) {
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
		{name: "text short", msg: &Message{Type: MessageTypeText, Text: "hello"}, want: "hello"},
		{name: "text empty", msg: &Message{Type: MessageTypeText}, want: ""},
		{
			name: "text truncated",
			msg:  &Message{Type: MessageTypeText, Text: strings.Repeat("字", 51)},
			want: strings.Repeat("字", 50) + "...",
		},
		{name: "image", msg: &Message{Type: MessageTypeImage}, want: "[图片]"},
		{name: "emoji with value", msg: &Message{Type: MessageTypeEmoji, Emoji: ":)"}, want: ":)"},
		{name: "emoji without value", msg: &Message{Type: MessageTypeEmoji}, want: "[表情]"},
		{name: "sticker", msg: &Message{Type: MessageTypeSticker}, want: "[表情包]"},
		{name: "unknown", msg: &Message{Type: "audio"}, want: "[消息]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := BuildPreview(tt.msg); got != tt.want {
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

			if err := EvaluateDailySendLimit(tt.sentToday, tt.repliedToday); !errors.Is(err, tt.wantErr) {
				t.Fatalf("EvaluateDailySendLimit(%v, %v) = %v, want %v",
					tt.sentToday, tt.repliedToday, err, tt.wantErr)
			}
		})
	}
}

func TestNeedsDailySendLimitCheck(t *testing.T) {
	t.Parallel()

	if NeedsDailySendLimitCheck(0, 2) || NeedsDailySendLimitCheck(1, 0) || NeedsDailySendLimitCheck(4, 4) {
		t.Fatal("NeedsDailySendLimitCheck should skip incomplete or self pairs")
	}

	if !NeedsDailySendLimitCheck(1, 2) {
		t.Fatal("NeedsDailySendLimitCheck(1, 2) should require the check")
	}
}

func TestDayRange(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 26, 13, 45, 30, 500, time.FixedZone("CST", 8*3600))

	start, end := DayRange(now)
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
		MemberA: Member{UserID: 3, Unread: 1},
		MemberB: Member{UserID: 9, Unread: 4},
	}

	if peer, unread := conv.PeerAndUnread(3); peer != 9 || unread != 1 {
		t.Fatalf("PeerAndUnread(3) = (%d, %d), want (9, 1)", peer, unread)
	}

	if peer, unread := conv.PeerAndUnread(9); peer != 3 || unread != 4 {
		t.Fatalf("PeerAndUnread(9) = (%d, %d), want (3, 4)", peer, unread)
	}

	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	conv.CreatedAt = created

	if got := conv.DisplayTime(); !got.Equal(created) {
		t.Fatalf("DisplayTime with zero UpdatedAt = %v, want %v", got, created)
	}

	updated := created.Add(time.Hour)
	conv.UpdatedAt = updated

	if got := conv.DisplayTime(); !got.Equal(updated) {
		t.Fatalf("DisplayTime = %v, want %v", got, updated)
	}
}
