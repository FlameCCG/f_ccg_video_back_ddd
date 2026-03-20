package entity

import (
	"errors"
	"strings"
	"testing"
	"time"

	chaterrors "f_ccg_video_back_ddd/internal/domain/chat/domainerr"
	chatservice "f_ccg_video_back_ddd/internal/domain/chat/service"
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
		{name: "no sender", senderID: 0, receiverID: 2, wantErr: chaterrors.ErrNotLoggedIn},
		{name: "no receiver", senderID: 1, receiverID: 0, wantErr: chaterrors.ErrNotLoggedIn},
		{name: "self", senderID: 5, receiverID: 5, wantErr: chaterrors.ErrSelfMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := (&Message{SenderID: tt.senderID, ReceiverID: tt.receiverID}).EnsureParticipants(); !errors.Is(err, tt.wantErr) {
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
		{name: "ok", userID: 1, peerID: 2, selfErr: chaterrors.ErrInvalidPeerID, wantErr: nil},
		{name: "not logged in", userID: 0, peerID: 2, selfErr: chaterrors.ErrInvalidPeerID, wantErr: chaterrors.ErrNotLoggedIn},
		{name: "empty peer", userID: 1, peerID: 0, selfErr: chaterrors.ErrInvalidPeerID, wantErr: chaterrors.ErrEmptyPeerID},
		{name: "self on read path", userID: 3, peerID: 3, selfErr: chaterrors.ErrInvalidPeerID, wantErr: chaterrors.ErrInvalidPeerID},
		{name: "self on create path", userID: 3, peerID: 3, selfErr: chaterrors.ErrSelfConversation, wantErr: chaterrors.ErrSelfConversation},
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
		{name: "text empty", msgType: MessageTypeText, wantErr: chaterrors.ErrEmptyText},
		{name: "text too long", msgType: MessageTypeText, text: strings.Repeat("字", 1001), wantErr: chaterrors.ErrTextTooLong},
		{name: "text at limit", msgType: MessageTypeText, text: strings.Repeat("字", 1000), wantErr: nil},
		{name: "emoji ok", msgType: MessageTypeEmoji, emoji: ":)", wantErr: nil},
		{name: "emoji falls back to text", msgType: MessageTypeEmoji, text: ":)", wantErr: nil},
		{name: "emoji empty", msgType: MessageTypeEmoji, wantErr: chaterrors.ErrEmptyEmoji},
		{name: "image ok", msgType: MessageTypeImage, media: &Media{URL: "http://x/a.png"}, wantErr: nil},
		{name: "image nil media", msgType: MessageTypeImage, wantErr: chaterrors.ErrEmptyMediaURL},
		{name: "sticker empty url", msgType: MessageTypeSticker, media: &Media{}, wantErr: chaterrors.ErrEmptyMediaURL},
		{name: "unknown type", msgType: "video", wantErr: chaterrors.ErrUnsupportedMessageType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := (&Message{Type: tt.msgType, Text: tt.text, Emoji: tt.emoji, Media: tt.media}).ValidateContent(); !errors.Is(err, tt.wantErr) {
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
		{name: "sent without reply", sentToday: true, repliedToday: false, wantErr: chaterrors.ErrDailySendLimit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := chatservice.EvaluateDailySendLimit(tt.sentToday, tt.repliedToday); !errors.Is(err, tt.wantErr) {
				t.Fatalf("EvaluateDailySendLimit(%v, %v) = %v, want %v",
					tt.sentToday, tt.repliedToday, err, tt.wantErr)
			}
		})
	}
}

func TestNeedsDailySendLimitCheck(t *testing.T) {
	t.Parallel()

	if chatservice.NeedsDailySendLimitCheck(0, 2) || chatservice.NeedsDailySendLimitCheck(1, 0) || chatservice.NeedsDailySendLimitCheck(4, 4) {
		t.Fatal("NeedsDailySendLimitCheck should skip incomplete or self pairs")
	}

	if !chatservice.NeedsDailySendLimitCheck(1, 2) {
		t.Fatal("NeedsDailySendLimitCheck(1, 2) should require the check")
	}
}

func TestDayRange(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 26, 13, 45, 30, 500, time.FixedZone("CST", 8*3600))

	start, end := chatservice.DayRange(now)
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
