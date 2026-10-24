package chat

import (
	"context"
	"golang.org/x/sync/errgroup"
	"os"
	"testing"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/chat"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mongopo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CHAT_TEST_MONGO_URI points to a disposable transaction-capable Mongo instance.
// This test creates and drops only its own unique database.
func TestSendMessageRealMongoTransaction(t *testing.T) {
	uri := os.Getenv("CHAT_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("CHAT_TEST_MONGO_URI not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	db := client.Database("codex_chat_audit_" + primitive.NewObjectID().Hex())
	defer db.Drop(context.Background())
	if err := db.CreateCollection(ctx, chatMessagesCol); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateCollection(ctx, chatConversationsCol, options.CreateCollection().SetValidator(bson.M{"requiredNeverPresent": bson.M{"$exists": true}})); err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	msg := &domain.Message{ID: primitive.NewObjectID().Hex(), ConversationID: "1_2", SenderID: 1, ReceiverID: 2, Type: domain.MessageTypeText, Text: "transaction", CreatedAt: time.Now().Truncate(time.Millisecond)}
	if err := store.SendMessage(ctx, msg, "transaction"); err == nil {
		t.Fatal("summary validation must fail")
	}
	if count, err := db.Collection(chatMessagesCol).CountDocuments(ctx, bson.M{}); err != nil || count != 0 {
		t.Fatalf("failed summary left messages: count=%d error=%v", count, err)
	}
	if err := db.RunCommand(ctx, bson.D{{Key: "collMod", Value: chatConversationsCol}, {Key: "validator", Value: bson.M{}}}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := store.SendMessage(ctx, msg, "transaction"); err != nil {
		t.Fatalf("retry after aborted insert: %v", err)
	}
	if err := store.SendMessage(ctx, msg, "transaction"); err != nil {
		t.Fatalf("duplicate message: %v", err)
	}
	if count, err := db.Collection(chatMessagesCol).CountDocuments(ctx, bson.M{}); err != nil || count != 1 {
		t.Fatalf("messages=%d error=%v", count, err)
	}
	var conv mongopo.ChatConversation
	if err := db.Collection(chatConversationsCol).FindOne(ctx, bson.M{"_id": "1_2"}).Decode(&conv); err != nil {
		t.Fatal(err)
	}
	if unread := conv.MemberB.Unread; unread != 1 {
		t.Fatalf("duplicate send incremented unread: %v", conv)
	}
	concurrent := *msg
	concurrent.ID = primitive.NewObjectID().Hex()
	var group errgroup.Group
	for range 6 {
		group.Go(func() error { copy := concurrent; return store.SendMessage(ctx, &copy, "transaction") })
	}
	if err := group.Wait(); err != nil {
		t.Fatalf("concurrent duplicate: %v", err)
	}
	if count, err := db.Collection(chatMessagesCol).CountDocuments(ctx, bson.M{}); err != nil || count != 2 {
		t.Fatalf("concurrent messages=%d error=%v", count, err)
	}
	if err := db.Collection(chatConversationsCol).FindOne(ctx, bson.M{"_id": "1_2"}).Decode(&conv); err != nil {
		t.Fatal(err)
	}
	if conv.MemberB.Unread != 2 {
		t.Fatalf("concurrent duplicate incremented unread: %v", conv)
	}
}
