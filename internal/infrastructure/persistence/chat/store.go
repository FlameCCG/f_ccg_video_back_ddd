package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/chat"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mongopo"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Store 私信仓储的 Mongo 实现。
//
// 同时实现领域写侧仓储 chat.Repository 与应用读侧端口 chat.Query：
// 两者共用同一份集合名、字段名与索引 hint。
// db 可能为 nil（Mongo 未配置），此时 Available 返回 false，
// 应用服务据此返回「mongo 未初始化」，与原实现的降级行为一致。
type Store struct {
	db *mongo.Database
}

var _ domain.Repository = (*Store)(nil)

// NewStore 构造私信 Mongo 仓储；db 允许为 nil。
func NewStore(db *mongo.Database) *Store {
	return &Store{db: db}
}

// Available 报告 Mongo 是否可用。
func (s *Store) Available() bool {
	return s != nil && s.db != nil
}

// EnsureIndexes 创建聊天消息与会话集合索引（幂等）。
func (s *Store) EnsureIndexes(ctx context.Context) error {
	if !s.Available() {
		return domain.ErrStoreUnavailable
	}

	{
		col := s.db.Collection(chatMessagesCol)

		_, err := col.Indexes().CreateMany(ctx, []mongo.IndexModel{
			{
				Keys: bson.D{
					{Key: fieldConversationID, Value: 1},
					{Key: fieldID, Value: -1},
				},
				Options: options.Index().SetName(indexConversationIDDesc),
			},
			{
				Keys: bson.D{
					{Key: fieldConversationID, Value: 1},
					{Key: fieldSenderID, Value: 1},
					{Key: fieldReceiverID, Value: 1},
					{Key: fieldCreatedAt, Value: -1},
				},
				Options: options.Index().SetName(indexConversationSenderReceiver),
			},
			{
				Keys: bson.D{
					{Key: fieldReceiverID, Value: 1},
					{Key: fieldID, Value: -1},
				},
				Options: options.Index().SetName(indexReceiverIDDesc),
			},
		})
		if err != nil {
			return err
		}
	}

	{
		col := s.db.Collection(chatConversationsCol)

		_, err := col.Indexes().CreateMany(ctx, []mongo.IndexModel{
			{
				Keys: bson.D{
					{Key: fieldParticipants, Value: 1},
					{Key: fieldUpdatedAt, Value: -1},
				},
				Options: options.Index().SetName(indexParticipantsUpdatedAtDesc),
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// InsertMessage 落库一条消息并回填 ObjectID 的十六进制表示。
func (s *Store) InsertMessage(ctx context.Context, msg *domain.Message) error {
	if !s.Available() {
		return domain.ErrStoreUnavailable
	}

	if msg == nil {
		return nil
	}

	oid := primitive.NewObjectID()
	row := mongopo.ChatMessage{
		ID:             oid,
		ConversationID: msg.ConversationID,
		SenderID:       msg.SenderID,
		ReceiverID:     msg.ReceiverID,
		Type:           msg.Type,
		Text:           msg.Text,
		Emoji:          msg.Emoji,
		Media:          toPOMedia(msg.Media),
		CreatedAt:      msg.CreatedAt,
	}

	if _, err := s.db.Collection(chatMessagesCol).InsertOne(ctx, row); err != nil {
		return err
	}

	msg.ID = oid.Hex()

	return nil
}

// ApplySend 按消息推进会话：写入 lastMessage、双方可见、接收方未读 +1（upsert）。
func (s *Store) ApplySend(ctx context.Context, msg *domain.Message, preview string) error {
	if !s.Available() {
		return domain.ErrStoreUnavailable
	}

	if msg == nil {
		return nil
	}

	msgID, err := primitive.ObjectIDFromHex(msg.ID)
	if err != nil {
		return fmt.Errorf("非法消息 ID %q: %w", msg.ID, err)
	}

	aID, bID := domain.SortedPair(msg.SenderID, msg.ReceiverID)
	conversationID := domain.BuildConversationID(aID, bID)

	receiverSlot := domain.SlotOf(msg.ReceiverID, msg.SenderID)
	senderSlot := receiverSlot.Other()

	receiverUnreadPath := receiverSlot.String() + suffixUnread
	receiverVisiblePath := receiverSlot.String() + suffixVisible
	senderUnreadPath := senderSlot.String() + suffixUnread
	senderVisiblePath := senderSlot.String() + suffixVisible

	now := msg.CreatedAt
	if now.IsZero() {
		now = time.Now()
	}

	last := mongopo.ChatConversationLastMessage{
		MsgID:     msgID,
		Type:      msg.Type,
		Preview:   preview,
		SenderID:  msg.SenderID,
		CreatedAt: now,
	}

	update := bson.D{
		{Key: opSetOnInsert, Value: bson.D{
			{Key: fieldParticipants, Value: bson.A{aID, bID}},
			{Key: fieldMemberAUserID, Value: aID},
			{Key: fieldMemberBUserID, Value: bID},
			{Key: senderUnreadPath, Value: int64(0)},
			{Key: fieldCreatedAt, Value: now},
		}},
		{Key: opSet, Value: bson.D{
			{Key: fieldLastMessage, Value: last},
			{Key: fieldUpdatedAt, Value: now},
			{Key: senderVisiblePath, Value: true},
			{Key: receiverVisiblePath, Value: true},
		}},
		{Key: opInc, Value: bson.D{{Key: receiverUnreadPath, Value: int64(1)}}},
	}

	_, err = s.db.Collection(chatConversationsCol).UpdateByID(
		ctx,
		conversationID,
		update,
		options.Update().SetUpsert(true),
	)

	return err
}

// UpsertConversation 幂等创建/恢复会话，并返回会话当前状态。
func (s *Store) UpsertConversation(
	ctx context.Context,
	userID, peerID uint,
	now time.Time,
) (*domain.Conversation, error) {
	if !s.Available() {
		return nil, domain.ErrStoreUnavailable
	}

	aID, bID := domain.SortedPair(userID, peerID)
	conversationID := domain.BuildConversationID(aID, bID)

	mySlot := domain.SlotOf(userID, peerID)
	peerSlot := mySlot.Other()

	update := bson.D{
		{Key: opSetOnInsert, Value: bson.D{
			{Key: fieldParticipants, Value: bson.A{aID, bID}},
			{Key: fieldMemberAUserID, Value: aID},
			{Key: fieldMemberAUnread, Value: int64(0)},
			{Key: fieldMemberBUserID, Value: bID},
			{Key: fieldMemberBUnread, Value: int64(0)},
			{Key: peerSlot.String() + suffixVisible, Value: false},
			{Key: fieldCreatedAt, Value: now},
		}},
		{Key: opSet, Value: bson.D{
			{Key: fieldUpdatedAt, Value: now},
			{Key: mySlot.String() + suffixVisible, Value: true},
		}},
	}

	var conv mongopo.ChatConversation

	err := s.db.Collection(chatConversationsCol).
		FindOneAndUpdate(
			ctx,
			bson.M{fieldID: conversationID},
			update,
			options.FindOneAndUpdate().
				SetUpsert(true).
				SetReturnDocument(options.After).
				SetProjection(bson.M{
					fieldID:            1,
					fieldLastMessage:   1,
					fieldCreatedAt:     1,
					fieldUpdatedAt:     1,
					fieldMemberAUserID: 1,
					fieldMemberAUnread: 1,
					fieldMemberBUserID: 1,
					fieldMemberBUnread: 1,
				}),
		).
		Decode(&conv)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrConversationNotFound
		}

		return nil, err
	}

	return toDomainConversation(&conv), nil
}

// HideConversation 仅对 userID 隐藏会话并清零其未读，保留对端会话与历史消息。
func (s *Store) HideConversation(ctx context.Context, userID, peerID uint, now time.Time) error {
	if !s.Available() {
		return domain.ErrStoreUnavailable
	}

	conversationID := domain.BuildConversationID(userID, peerID)
	slot := domain.SlotOf(userID, peerID).String()

	_, err := s.db.Collection(chatConversationsCol).UpdateOne(
		ctx,
		bson.M{fieldID: conversationID},
		bson.M{
			opSet: bson.M{
				slot + suffixVisible: false,
				slot + suffixUnread:  int64(0),
				fieldUpdatedAt:       now,
			},
		},
	)

	return err
}

// MarkRead 清零 userID 在该会话中的未读并记录已读时间；返回是否命中会话。
func (s *Store) MarkRead(ctx context.Context, userID, peerID uint, now time.Time) (bool, error) {
	if !s.Available() {
		return false, domain.ErrStoreUnavailable
	}

	conversationID := domain.BuildConversationID(userID, peerID)
	// 根据 deterministic 的 memberA/memberB 更新 unread
	slot := domain.SlotOf(userID, peerID).String()

	update := bson.D{{Key: opSet, Value: bson.D{
		{Key: slot + suffixUnread, Value: int64(0)},
		{Key: slot + suffixLastReadAt, Value: now},
	}}}

	res, err := s.db.Collection(chatConversationsCol).UpdateByID(ctx, conversationID, update)
	if err != nil {
		return false, err
	}

	return res.MatchedCount != 0, nil
}

// ExistsMessageBetween 判断指定时间窗内是否存在 senderID → receiverID 的消息。
func (s *Store) ExistsMessageBetween(
	ctx context.Context,
	conversationID string,
	senderID, receiverID uint,
	start, end time.Time,
) (bool, error) {
	if !s.Available() || conversationID == "" || senderID == 0 || receiverID == 0 {
		return false, nil
	}

	filter := bson.M{
		fieldConversationID: conversationID,
		fieldSenderID:       senderID,
		fieldReceiverID:     receiverID,
		fieldCreatedAt: bson.M{
			"$gte": start,
			"$lt":  end,
		},
	}

	err := s.db.Collection(chatMessagesCol).FindOne(
		ctx,
		filter,
		options.FindOne().
			SetProjection(bson.M{fieldID: 1}).
			SetHint(indexConversationSenderReceiver),
	).Err()
	if err == nil {
		return true, nil
	}

	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}

	return false, err
}

// visibleConversationFilter 可见会话过滤条件：参与者本人，且有历史消息或自己未隐藏。
func visibleConversationFilter(userID uint) bson.M {
	return bson.M{
		fieldParticipants: userID,
		"$or": bson.A{
			bson.M{fieldLastMessage: bson.M{"$exists": true}},
			bson.M{fieldMemberAUserID: userID, "memberA" + suffixVisible: true},
			bson.M{fieldMemberBUserID: userID, "memberB" + suffixVisible: true},
		},
	}
}
