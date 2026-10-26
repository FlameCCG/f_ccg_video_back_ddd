package chat

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/chat"

	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

// 原 chat_service 的每个方法都以 getChatDB() 开头，Mongo 未初始化时返回
// 「mongo 未初始化」。新架构把该判定收敛到 Store.Available()，
// 本文件锁定 db == nil 时的降级契约（离线测试，不依赖真实 Mongo）。

func TestStoreAvailableReportsNilHandles(t *testing.T) {
	t.Parallel()

	if NewStore(nil).Available() {
		t.Fatal("nil database handle must report unavailable")
	}

	var nilStore *Store
	if nilStore.Available() {
		t.Fatal("nil store must report unavailable")
	}
}

// TestStoreWriteMethodsReturnStoreUnavailable 写侧方法在 Mongo 缺失时统一返回
// domain.ErrStoreUnavailable（文案「mongo 未初始化」会原样透给前端）。
func TestStoreWriteMethodsReturnStoreUnavailable(t *testing.T) {
	t.Parallel()

	store := NewStore(nil)
	ctx := context.Background()
	now := time.Now()

	t.Run("EnsureIndexes", func(t *testing.T) {
		t.Parallel()

		if err := store.EnsureIndexes(ctx); !errors.Is(err, domain.ErrStoreUnavailable) {
			t.Fatalf("err = %v, want ErrStoreUnavailable", err)
		}
	})

	t.Run("InsertMessage", func(t *testing.T) {
		t.Parallel()

		if err := store.InsertMessage(ctx, &domain.Message{}); !errors.Is(err, domain.ErrStoreUnavailable) {
			t.Fatalf("err = %v, want ErrStoreUnavailable", err)
		}
	})

	t.Run("ApplySend", func(t *testing.T) {
		t.Parallel()

		if err := store.ApplySend(ctx, &domain.Message{}, "预览"); !errors.Is(err, domain.ErrStoreUnavailable) {
			t.Fatalf("err = %v, want ErrStoreUnavailable", err)
		}
	})

	t.Run("UpsertConversation", func(t *testing.T) {
		t.Parallel()

		if _, err := store.UpsertConversation(ctx, 1, 2, now); !errors.Is(err, domain.ErrStoreUnavailable) {
			t.Fatalf("err = %v, want ErrStoreUnavailable", err)
		}
	})

	t.Run("HideConversation", func(t *testing.T) {
		t.Parallel()

		if err := store.HideConversation(ctx, 1, 2, now); !errors.Is(err, domain.ErrStoreUnavailable) {
			t.Fatalf("err = %v, want ErrStoreUnavailable", err)
		}
	})

	t.Run("MarkRead", func(t *testing.T) {
		t.Parallel()

		if _, err := store.MarkRead(ctx, 1, 2, now); !errors.Is(err, domain.ErrStoreUnavailable) {
			t.Fatalf("err = %v, want ErrStoreUnavailable", err)
		}
	})
}

// TestStoreReadMethodsReturnStoreUnavailable 读侧查询在 Mongo 缺失时同样返回
// domain.ErrStoreUnavailable。
func TestStoreReadMethodsReturnStoreUnavailable(t *testing.T) {
	t.Parallel()

	store := NewStore(nil)
	ctx := context.Background()

	if _, _, err := store.ListConversations(ctx, 1, 1, 20); !errors.Is(err, domain.ErrStoreUnavailable) {
		t.Fatalf("ListConversations err = %v, want ErrStoreUnavailable", err)
	}

	if _, err := store.TotalUnread(ctx, 1); !errors.Is(err, domain.ErrStoreUnavailable) {
		t.Fatalf("TotalUnread err = %v, want ErrStoreUnavailable", err)
	}

	if _, err := store.ListMessagesBefore(ctx, 1, 2, "", 11); !errors.Is(err, domain.ErrStoreUnavailable) {
		t.Fatalf("ListMessagesBefore err = %v, want ErrStoreUnavailable", err)
	}
}

// TestExistsMessageBetweenDegradesToFalse 频率限制查询在 Mongo 缺失或参数不全时
// 返回 (false, nil)——原实现据此直接放行发送，不因存储不可用而拒绝用户。
func TestExistsMessageBetweenDegradesToFalse(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	start := time.Now()
	end := start.Add(24 * time.Hour)

	tests := []struct {
		name           string
		store          *Store
		conversationID string
		senderID       uint
		receiverID     uint
	}{
		{name: "mongo missing", store: NewStore(nil), conversationID: "1_2", senderID: 1, receiverID: 2},
		{name: "empty conversation id", store: NewStore(nil), conversationID: "", senderID: 1, receiverID: 2},
		{name: "no sender", store: NewStore(nil), conversationID: "1_2", senderID: 0, receiverID: 2},
		{name: "no receiver", store: NewStore(nil), conversationID: "1_2", senderID: 1, receiverID: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exists, err := tt.store.ExistsMessageBetween(
				ctx, tt.conversationID, tt.senderID, tt.receiverID, start, end,
			)
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}

			if exists {
				t.Fatal("exists = true, want false")
			}
		})
	}
}

// TestVisibleConversationFilterShape 可见会话过滤条件：必须是参与者本人，
// 且「有历史消息」或「自己那一侧仍可见」。字段路径属于存储契约。
func TestVisibleConversationFilterShape(t *testing.T) {
	t.Parallel()

	filter := visibleConversationFilter(7)

	if got := filter[fieldParticipants]; got != uint(7) {
		t.Fatalf("participants = %v, want 7", got)
	}

	or, ok := filter["$or"]
	if !ok {
		t.Fatalf("filter must contain $or: %+v", filter)
	}

	// bson.A 是 primitive.A（命名类型），按其自身类型断言。
	list, ok := or.(bson.A)
	if !ok {
		t.Fatalf("$or = %T, want bson.A", or)
	}

	if len(list) != 3 {
		t.Fatalf("$or has %d clauses, want 3", len(list))
	}
}

// TestFieldConstantsMatchStorageContract Mongo 集合名与字段路径是存储契约，
// 与原 service/chat_service 逐字一致。
func TestFieldConstantsMatchStorageContract(t *testing.T) {
	t.Parallel()

	pairs := map[string]string{
		chatMessagesCol:                 "chat_messages",
		chatConversationsCol:            "chat_conversations",
		fieldConversationID:             "conversationId",
		fieldID:                         "_id",
		fieldCreatedAt:                  "createdAt",
		fieldUpdatedAt:                  "updatedAt",
		fieldParticipants:               "participants",
		fieldLastMessage:                "lastMessage",
		fieldSenderID:                   "senderId",
		fieldReceiverID:                 "receiverId",
		fieldMemberAUserID:              "memberA.userId",
		fieldMemberBUserID:              "memberB.userId",
		fieldMemberAUnread:              "memberA.unread",
		fieldMemberBUnread:              "memberB.unread",
		opSet:                           "$set",
		opSetOnInsert:                   "$setOnInsert",
		opInc:                           "$inc",
		exprMemberAUserID:               "$memberA.userId",
		indexConversationIDDesc:         "conversationId_id_desc",
		indexConversationSenderReceiver: "conversationId_sender_receiver_createdAt_desc",
		indexReceiverIDDesc:             "receiverId_id_desc",
		indexParticipantsUpdatedAtDesc:  "participants_updatedAt_desc",
	}

	for got, want := range pairs {
		if got != want {
			t.Fatalf("storage contract drift: got %q, want %q", got, want)
		}
	}

	// 成员字段前缀 + 后缀拼出的路径必须与常量一致。
	if got := domain.MemberA.String() + suffixUnread; got != fieldMemberAUnread {
		t.Fatalf("memberA unread path = %q, want %q", got, fieldMemberAUnread)
	}

	if got := domain.MemberB.String() + suffixVisible; got != "memberB.visible" {
		t.Fatalf("memberB visible path = %q", got)
	}

	if got := domain.MemberA.String() + suffixLastReadAt; got != "memberA.lastReadAt" {
		t.Fatalf("memberA lastReadAt path = %q", got)
	}
}

// TestFollowRelationQueryDegradesWithoutDB 关注关系读侧投影在 db 缺失时返回
// (false, nil)——与原 global.DB == nil 的降级行为一致（不阻断发送）。
func TestFollowRelationQueryDegradesWithoutDB(t *testing.T) {
	t.Parallel()

	query := NewFollowRelationQuery(nil)
	ctx := context.Background()

	tests := []struct {
		name         string
		userID       uint
		targetUserID uint
	}{
		{name: "db missing", userID: 1, targetUserID: 2},
		{name: "no user", userID: 0, targetUserID: 2},
		{name: "no target", userID: 1, targetUserID: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			following, err := query.IsFollowing(ctx, tt.userID, tt.targetUserID)
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}

			if following {
				t.Fatal("following = true, want false")
			}
		})
	}
}

// TestIndexInitializerSkipsWhenMongoMissing Mongo 缺失时索引初始化组件静默跳过
// （原实现此时根本走不到 EnsureIndexes），不执行索引创建。
func TestIndexInitializerSkipsWhenMongoMissing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	for _, initializer := range []*IndexInitializer{
		NewIndexInitializer(nil, zap.NewNop()),
		NewIndexInitializer(NewStore(nil), zap.NewNop()),
	} {
		initializer.Start(ctx)
	}
}

// TestIndexInitializerSwallowsEnsureIndexesFailure 索引创建失败只告警、不阻断启动。
func TestIndexInitializerSwallowsEnsureIndexesFailure(t *testing.T) {
	t.Parallel()

	repo := &failingRepository{}
	NewIndexInitializer(repo, zap.NewNop()).Start(context.Background())

	if !repo.called {
		t.Fatal("EnsureIndexes must be attempted when the store reports available")
	}
}

// failingRepository 是一个「可用但建索引总失败」的仓储桩。
type failingRepository struct {
	domain.Repository

	called bool
}

func (r *failingRepository) Available() bool { return true }

func (r *failingRepository) EnsureIndexes(context.Context) error {
	r.called = true

	return errors.New("create index failed")
}
