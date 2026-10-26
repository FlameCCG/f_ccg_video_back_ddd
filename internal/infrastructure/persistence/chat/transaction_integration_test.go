package chat

import (
	"context"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mongopo"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	domain "f_ccg_video_back_ddd/internal/domain/chat"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CHAT_TEST_MONGO_URI points to a disposable transaction-capable Mongo instance.
// This test creates and drops only its own unique database.
func TestSendMessageRealMongoTransaction(t *testing.T) {
	t.Parallel()

	uri := os.Getenv("CHAT_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("CHAT_TEST_MONGO_URI not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	require.NoError(t, err)

	defer func() { require.NoError(t, client.Disconnect(context.Background())) }()

	db := client.Database("codex_chat_audit_" + primitive.NewObjectID().Hex())
	defer func() { require.NoError(t, db.Drop(context.Background())) }()

	{
		err := db.CreateCollection(ctx, chatMessagesCol)
		require.NoError(t, err)
	}

	{
		err := db.CreateCollection(ctx, chatConversationsCol, options.CreateCollection().SetValidator(bson.M{"requiredNeverPresent": bson.M{"$exists": true}}))
		require.NoError(t, err)
	}

	store := NewStore(db)

	msg := &domain.Message{ID: primitive.NewObjectID().Hex(), ConversationID: "1_2", SenderID: 1, ReceiverID: 2, Type: domain.MessageTypeText, Text: "transaction", CreatedAt: time.Now().Truncate(time.Millisecond)}
	{
		err := store.SendMessage(ctx, msg, "transaction")
		require.Error(t, err, "summary validation must fail")
	}

	{
		count, err := db.Collection(chatMessagesCol).CountDocuments(ctx, bson.M{})
		require.NoError(t, err, "failed summary left messages: count=%d error=%v", count, err)
		require.EqualValues(t, 0, count, "failed summary left messages: count=%d error=%v", count, err)
	}

	{
		err := db.RunCommand(ctx, bson.D{{Key: "collMod", Value: chatConversationsCol}, {Key: "validator", Value: bson.M{}}}).Err()
		require.NoError(t, err)
	}

	{
		err := store.SendMessage(ctx, msg, "transaction")
		require.NoError(t, err, "retry after aborted insert: %v", err)
	}

	{
		err := store.SendMessage(ctx, msg, "transaction")
		require.NoError(t, err, "duplicate message: %v", err)
	}

	{
		count, err := db.Collection(chatMessagesCol).CountDocuments(ctx, bson.M{})
		require.NoError(t, err, "messages=%d error=%v", count, err)
		require.EqualValues(t, 1, count, "messages=%d error=%v", count, err)
	}

	var conv mongopo.ChatConversation
	{
		err := db.Collection(chatConversationsCol).FindOne(ctx, bson.M{fieldID: "1_2"}).Decode(&conv)
		require.NoError(t, err)
	}

	{
		unread := conv.MemberB.Unread
		require.EqualValues(t, 1, unread, "duplicate send incremented unread: %v", conv)
	}

	concurrent := *msg
	concurrent.ID = primitive.NewObjectID().Hex()

	var group errgroup.Group
	for range 6 {
		group.Go(func() error { message := concurrent; return store.SendMessage(ctx, &message, "transaction") })
	}

	{
		err := group.Wait()
		require.NoError(t, err, "concurrent duplicate: %v", err)
	}

	{
		count, err := db.Collection(chatMessagesCol).CountDocuments(ctx, bson.M{})
		require.NoError(t, err, "concurrent messages=%d error=%v", count, err)
		require.EqualValues(t, 2, count, "concurrent messages=%d error=%v", count, err)
	}

	{
		err := db.Collection(chatConversationsCol).FindOne(ctx, bson.M{fieldID: "1_2"}).Decode(&conv)
		require.NoError(t, err)
	}

	require.EqualValues(t, 2, conv.MemberB.Unread, "concurrent duplicate incremented unread: %v", conv)
}
