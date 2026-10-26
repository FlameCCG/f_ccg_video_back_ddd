package chat

import (
	"context"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/chat"
)

// 本文件是私信应用层单测用的手写 stub：不走 wire、不依赖真实 Mongo/MySQL/Redis。

// messageWindow 记录一次 ExistsMessageBetween 查询的方向。
type messageWindow struct {
	conversationID string
	senderID       uint
	receiverID     uint
}

type fakeRepository struct {
	available bool

	insertErr error
	applyErr  error
	upsert    *domain.Conversation
	upsertErr error
	hideErr   error
	matched   bool
	markErr   error

	// exists 按「发送方→接收方」方向给出是否已有消息。
	exists    map[messageWindow]bool
	existsErr error

	insertedID string
	inserted   []domain.Message
	applied    []domain.Message
	previews   []string
	hidden     []messageWindow
	markedRead []messageWindow
	windows    []messageWindow
}

func (f *fakeRepository) Available() bool { return f.available }

func (f *fakeRepository) EnsureIndexes(context.Context) error { return nil }

func (f *fakeRepository) InsertMessage(_ context.Context, msg *domain.Message) error {
	if f.insertErr != nil {
		return f.insertErr
	}

	if f.insertedID == "" {
		f.insertedID = "652f1a2b3c4d5e6f70819200"
	}

	msg.ID = f.insertedID
	f.inserted = append(f.inserted, *msg)

	return nil
}

func (f *fakeRepository) ApplySend(_ context.Context, msg *domain.Message, preview string) error {
	if f.applyErr != nil {
		return f.applyErr
	}

	f.applied = append(f.applied, *msg)
	f.previews = append(f.previews, preview)

	return nil
}

func (f *fakeRepository) UpsertConversation(
	_ context.Context,
	_, _ uint,
	_ time.Time,
) (*domain.Conversation, error) {
	return f.upsert, f.upsertErr
}

func (f *fakeRepository) HideConversation(_ context.Context, userID, peerID uint, _ time.Time) error {
	f.hidden = append(f.hidden, messageWindow{senderID: userID, receiverID: peerID})

	return f.hideErr
}

func (f *fakeRepository) MarkRead(_ context.Context, userID, peerID uint, _ time.Time) (bool, error) {
	f.markedRead = append(f.markedRead, messageWindow{senderID: userID, receiverID: peerID})

	return f.matched, f.markErr
}

func (f *fakeRepository) ExistsMessageBetween(
	_ context.Context,
	conversationID string,
	senderID, receiverID uint,
	_, _ time.Time,
) (bool, error) {
	window := messageWindow{conversationID: conversationID, senderID: senderID, receiverID: receiverID}
	f.windows = append(f.windows, window)

	if f.existsErr != nil {
		return false, f.existsErr
	}

	return f.exists[window], nil
}

type fakeQuery struct {
	conversations []ConversationItem
	total         int64
	listErr       error

	unread    int64
	unreadErr error

	messages []MessageItem
	msgErr   error

	lastPage     int
	lastPageSize int
	lastBefore   string
	lastLimit    int
}

func (f *fakeQuery) ListConversations(
	_ context.Context,
	_ uint,
	page, pageSize int,
) ([]ConversationItem, int64, error) {
	f.lastPage, f.lastPageSize = page, pageSize

	return f.conversations, f.total, f.listErr
}

func (f *fakeQuery) TotalUnread(context.Context, uint) (int64, error) {
	return f.unread, f.unreadErr
}

func (f *fakeQuery) ListMessagesBefore(
	_ context.Context,
	_, _ uint,
	before string,
	limit int,
) ([]MessageItem, error) {
	f.lastBefore, f.lastLimit = before, limit

	return f.messages, f.msgErr
}

type fakeFollowRelation struct {
	following bool
	err       error
	calls     []messageWindow
}

func (f *fakeFollowRelation) IsFollowing(_ context.Context, userID, targetUserID uint) (bool, error) {
	f.calls = append(f.calls, messageWindow{senderID: userID, receiverID: targetUserID})

	return f.following, f.err
}

type fakeBadgeInvalidator struct {
	invalidated []int
}

func (f *fakeBadgeInvalidator) InvalidateCount(receiverID int) {
	f.invalidated = append(f.invalidated, receiverID)
}

func (f *fakeRepository) SendMessage(ctx context.Context, msg *domain.Message, preview string) error {
	before := len(f.inserted)
	if err := f.InsertMessage(ctx, msg); err != nil {
		return err
	}

	if err := f.ApplySend(ctx, msg, preview); err != nil {
		f.inserted = f.inserted[:before]
		return err
	}

	return nil
}
