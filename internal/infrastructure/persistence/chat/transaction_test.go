package chat

import (
	"context"
	"testing"
	"time"

	chatdomain "f_ccg_video_back_ddd/internal/domain/chat"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestSendMessageTransaction(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("summary failure aborts insertion", func(mt *mtest.T) {
		store := NewStore(mt.DB)
		mt.AddMockResponses(
			mtest.CreateCursorResponse(0, mt.DB.Name()+"."+chatMessagesCol, mtest.FirstBatch),
			mtest.CreateSuccessResponse(bson.E{Key: "n", Value: 1}),
			mtest.CreateCommandErrorResponse(mtest.CommandError{Code: 121, Message: "summary rejected"}),
			mtest.CreateSuccessResponse(),
		)
		msg := chatdomain.RestoreMessage(chatdomain.MessageSnapshot{ConversationID: "1_2", SenderID: 1, ReceiverID: 2, Type: chatdomain.MessageTypeText, Text: "hello", CreatedAt: time.Now()})
		if _, err := store.SendMessage(context.Background(), msg, "hello"); err == nil {
			mt.Fatal("expected summary error")
		}
		events := mt.GetAllStartedEvents()
		names := make([]string, 0, len(events))
		for _, event := range events {
			names = append(names, event.CommandName)
		}
		if len(names) != 4 || names[0] != "find" || names[1] != "insert" || names[2] != "update" || names[3] != "abortTransaction" {
			mt.Fatalf("commands = %v", names)
		}
		for _, event := range events[:3] {
			if event.Command.Lookup("autocommit").Boolean() {
				mt.Fatal("write escaped transaction")
			}
		}
	})
	mt.Run("same message ID does not increment unread again", func(mt *mtest.T) {
		oid := primitive.NewObjectID()
		now := time.Now().Truncate(time.Millisecond)
		mt.AddMockResponses(
			mtest.CreateCursorResponse(0, mt.DB.Name()+"."+chatMessagesCol, mtest.FirstBatch, bson.D{
				{Key: "_id", Value: oid}, {Key: fieldConversationID, Value: "1_2"}, {Key: fieldSenderID, Value: 1}, {Key: fieldReceiverID, Value: 2}, {Key: "type", Value: chatdomain.MessageTypeText}, {Key: "text", Value: "hello"}, {Key: fieldCreatedAt, Value: now},
			}),
			mtest.CreateSuccessResponse(),
		)
		msg := chatdomain.RestoreMessage(chatdomain.MessageSnapshot{ID: oid.Hex(), ConversationID: "1_2", SenderID: 1, ReceiverID: 2, Type: chatdomain.MessageTypeText, Text: "hello"})
		stored, err := NewStore(mt.DB).SendMessage(context.Background(), msg, "hello")
		if err != nil {
			mt.Fatal(err)
		}
		for _, event := range mt.GetAllStartedEvents() {
			if event.CommandName == "insert" || event.CommandName == "update" {
				mt.Fatalf("duplicate message wrote %s", event.CommandName)
			}
		}
		if !msg.CreatedAt().IsZero() {
			mt.Fatal("input message must stay unchanged")
		}
		if stored == nil || !stored.CreatedAt().Equal(now) {
			mt.Fatalf("created time = %v, want %v", stored.CreatedAt(), now)
		}
	})
}
