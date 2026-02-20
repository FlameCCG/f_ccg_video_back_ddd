package chat

import (
	"context"

	app "f_ccg_video_back_ddd/internal/application/chat"
	domain "f_ccg_video_back_ddd/internal/domain/chat"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mongopo"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var _ app.Query = (*Store)(nil)

// conversationRow 会话列表聚合投影行。
type conversationRow struct {
	ID          string                               `bson:"_id"`
	PeerID      uint                                 `bson:"peerId"`
	Unread      int64                                `bson:"unread"`
	LastMessage *mongopo.ChatConversationLastMessage `bson:"lastMessage"`
	UpdatedAt   primitive.DateTime                   `bson:"updatedAt"`
}

// conversationAggResult $facet 聚合结果。
type conversationAggResult struct {
	Rows []conversationRow `bson:"rows"`
	Meta []struct {
		Total int64 `bson:"total"`
	} `bson:"meta"`
}

// ListConversations 会话列表分页（按 updatedAt 倒序），返回读模型与总数。
func (s *Store) ListConversations(
	ctx context.Context,
	userID uint,
	page, pageSize int,
) ([]app.ConversationItem, int64, error) {
	if !s.Available() {
		return nil, 0, domain.ErrStoreUnavailable
	}

	col := s.db.Collection(chatConversationsCol)
	visibleFilter := visibleConversationFilter(userID)
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: visibleFilter}},
		{{
			Key: "$facet",
			Value: bson.D{
				{Key: "rows", Value: bson.A{
					bson.D{{Key: "$sort", Value: bson.D{{Key: fieldUpdatedAt, Value: -1}}}},
					bson.D{{Key: "$skip", Value: int64((page - 1) * pageSize)}},
					bson.D{{Key: "$limit", Value: int64(pageSize)}},
					bson.D{{
						Key: "$project",
						Value: bson.D{
							{Key: fieldID, Value: 1},
							{Key: fieldLastMessage, Value: 1},
							{Key: fieldUpdatedAt, Value: 1},
							{Key: "unread", Value: bson.D{{Key: "$cond", Value: bson.A{
								bson.D{{Key: "$eq", Value: bson.A{exprMemberAUserID, userID}}},
								exprMemberAUnread,
								exprMemberBUnread,
							}}}},
							{Key: "peerId", Value: bson.D{{Key: "$cond", Value: bson.A{
								bson.D{{Key: "$eq", Value: bson.A{exprMemberAUserID, userID}}},
								exprMemberBUserID,
								exprMemberAUserID,
							}}}},
						},
					}},
				}},
				{Key: "meta", Value: bson.A{
					bson.D{{Key: "$count", Value: "total"}},
				}},
			},
		}},
	}

	cur, err := col.Aggregate(ctx, pipeline, options.Aggregate().SetHint(indexParticipantsUpdatedAtDesc))
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = cur.Close(ctx) }()

	var result conversationAggResult
	if cur.Next(ctx) {
		if err := cur.Decode(&result); err != nil {
			return nil, 0, err
		}
	}

	if err := cur.Err(); err != nil {
		return nil, 0, err
	}

	total := int64(0)
	if len(result.Meta) > 0 {
		total = result.Meta[0].Total
	}

	out := make([]app.ConversationItem, 0, len(result.Rows))
	for _, r := range result.Rows {
		out = append(out, app.ConversationItem{
			ID:          r.ID,
			PeerID:      r.PeerID,
			Unread:      r.Unread,
			LastMessage: toLastMessageItem(r.LastMessage),
			UpdatedAt:   r.UpdatedAt.Time().Unix(),
		})
	}

	return out, total, nil
}

// TotalUnread 汇总用户在所有可见会话中的未读总数。
func (s *Store) TotalUnread(ctx context.Context, userID uint) (int64, error) {
	if !s.Available() {
		return 0, domain.ErrStoreUnavailable
	}

	col := s.db.Collection(chatConversationsCol)
	visibleFilter := visibleConversationFilter(userID)
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: visibleFilter}},
		{{Key: "$group", Value: bson.D{
			{Key: fieldID, Value: nil},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{
				bson.D{{Key: "$eq", Value: bson.A{exprMemberAUserID, userID}}},
				exprMemberAUnread,
				exprMemberBUnread,
			}}}}}},
		}}},
	}

	cur, err := col.Aggregate(ctx, pipeline, options.Aggregate().SetHint(indexParticipantsUpdatedAtDesc))
	if err != nil {
		return 0, err
	}
	defer func() { _ = cur.Close(ctx) }()

	var out struct {
		Total int64 `bson:"total"`
	}

	if cur.Next(ctx) {
		if err := cur.Decode(&out); err != nil {
			return 0, err
		}

		return out.Total, nil
	}

	return 0, nil
}

// ListMessagesBefore 按 before 游标向前拉取消息（按 _id 降序，最多 limit 条）。
func (s *Store) ListMessagesBefore(
	ctx context.Context,
	userID, peerID uint,
	before string,
	limit int,
) ([]app.MessageItem, error) {
	if !s.Available() {
		return nil, domain.ErrStoreUnavailable
	}

	conversationID := domain.BuildConversationID(userID, peerID)
	filter := bson.M{fieldConversationID: conversationID}

	if before != "" {
		oid, err := primitive.ObjectIDFromHex(before)
		if err != nil {
			return nil, domain.ErrInvalidCursor
		}

		filter[fieldID] = bson.M{"$lt": oid}
	}

	findOpt := options.Find().
		SetSort(bson.D{{Key: fieldID, Value: -1}}).
		SetLimit(int64(limit)).
		SetHint(indexConversationIDDesc)

	cur, err := s.db.Collection(chatMessagesCol).Find(ctx, filter, findOpt)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cur.Close(ctx) }()

	list := make([]app.MessageItem, 0, limit)

	for cur.Next(ctx) {
		var row mongopo.ChatMessage
		if err := cur.Decode(&row); err != nil {
			return nil, err
		}

		list = append(list, toMessageItem(&row))
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	return list, nil
}
