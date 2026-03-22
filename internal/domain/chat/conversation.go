package chat

import (
	"strconv"
	"time"
)

// MemberSlot 会话成员槽位。
//
// 两人会话的成员位置是确定性的：用户 ID 较小者固定落在 memberA，
// 较大者落在 memberB，从而使 conversationId 与成员字段路径可被双方一致推导。
type MemberSlot string

// 会话成员槽位取值（同时是 Mongo 文档中的字段前缀，属于存储契约）。
const (
	// MemberA 用户 ID 较小的一侧。
	MemberA MemberSlot = "memberA"
	// MemberB 用户 ID 较大的一侧。
	MemberB MemberSlot = "memberB"
)

// String 返回槽位对应的文档字段前缀。
func (s MemberSlot) String() string {
	return string(s)
}

// Other 返回对端槽位。
func (s MemberSlot) Other() MemberSlot {
	if s == MemberA {
		return MemberB
	}

	return MemberA
}

// Member 会话成员的未读与可见状态。作为快照与返回副本使用，字段保持公开。
type Member struct {
	UserID  uint
	Unread  int64
	Visible bool
}

// LastMessage 会话最后一条消息摘要。作为快照与返回副本使用，字段保持公开。
type LastMessage struct {
	// MsgID 消息 ID 的十六进制表示。
	MsgID     string
	Type      string
	Preview   string
	SenderID  uint
	CreatedAt time.Time
}

func (m *LastMessage) clone() *LastMessage {
	if m == nil {
		return nil
	}

	copied := *m

	return &copied
}

// ConversationSnapshot 是会话的持久化快照。字段公开，恢复时不重新执行创建校验。
type ConversationSnapshot struct {
	ID          string
	MemberA     Member
	MemberB     Member
	LastMessage *LastMessage
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Conversation 私信会话聚合（两人固定会话，ID 为确定性的 conversationKey）。
type Conversation struct {
	id          string
	memberA     Member
	memberB     Member
	lastMessage *LastMessage
	createdAt   time.Time
	updatedAt   time.Time
}

// RestoreConversation 从持久化快照恢复历史会话，不重新执行创建校验。
func RestoreConversation(snapshot ConversationSnapshot) *Conversation {
	return &Conversation{
		id:          snapshot.ID,
		memberA:     snapshot.MemberA,
		memberB:     snapshot.MemberB,
		lastMessage: snapshot.LastMessage.clone(),
		createdAt:   snapshot.CreatedAt,
		updatedAt:   snapshot.UpdatedAt,
	}
}

// Snapshot 返回会话快照的副本。修改返回值不会影响会话本身。
func (c *Conversation) Snapshot() ConversationSnapshot {
	if c == nil {
		return ConversationSnapshot{}
	}

	return ConversationSnapshot{
		ID:          c.id,
		MemberA:     c.memberA,
		MemberB:     c.memberB,
		LastMessage: c.lastMessage.clone(),
		CreatedAt:   c.createdAt,
		UpdatedAt:   c.updatedAt,
	}
}

// ID 返回会话 ID。
func (c *Conversation) ID() string {
	if c == nil {
		return ""
	}

	return c.id
}

// MemberA 返回较小用户 ID 一侧的成员副本。
func (c *Conversation) MemberA() Member {
	if c == nil {
		return Member{}
	}

	return c.memberA
}

// MemberB 返回较大用户 ID 一侧的成员副本。
func (c *Conversation) MemberB() Member {
	if c == nil {
		return Member{}
	}

	return c.memberB
}

// LastMessage 返回最后一条消息摘要的副本。
func (c *Conversation) LastMessage() *LastMessage {
	if c == nil {
		return nil
	}

	return c.lastMessage.clone()
}

// CreatedAt 返回创建时间。
func (c *Conversation) CreatedAt() time.Time {
	if c == nil {
		return time.Time{}
	}

	return c.createdAt
}

// UpdatedAt 返回更新时间。
func (c *Conversation) UpdatedAt() time.Time {
	if c == nil {
		return time.Time{}
	}

	return c.updatedAt
}

// PeerAndUnread 返回相对 userID 的对端用户 ID 与自己的未读数。
//
// memberA 命中则取 memberB 为对端，否则取 memberA 为对端。
func (c *Conversation) PeerAndUnread(userID uint) (uint, int64) {
	if c == nil {
		return 0, 0
	}

	if c.memberA.UserID == userID {
		return c.memberB.UserID, c.memberA.Unread
	}

	return c.memberA.UserID, c.memberB.Unread
}

// DisplayTime 返回展示用时间：updatedAt 为零值时回退 createdAt。
func (c *Conversation) DisplayTime() time.Time {
	if c == nil {
		return time.Time{}
	}

	if c.updatedAt.IsZero() {
		return c.createdAt
	}

	return c.updatedAt
}

// SortedPair 返回升序排列的两个用户 ID。
func (Conversation) SortedPair(a, b uint) (uint, uint) {
	if a <= b {
		return a, b
	}

	return b, a
}

// BuildConversationID 生成确定性会话 ID（"小ID_大ID"）。
func (c Conversation) BuildConversationID(a, b uint) string {
	x, y := c.SortedPair(a, b)

	return strconv.FormatUint(uint64(x), 10) + "_" + strconv.FormatUint(uint64(y), 10)
}

// SlotOf 返回 userID 在与 peerID 的会话中占据的成员槽位。
func (c Conversation) SlotOf(userID, peerID uint) MemberSlot {
	aID, _ := c.SortedPair(userID, peerID)
	if userID == aID {
		return MemberA
	}

	return MemberB
}

// RequireLoggedIn 校验当前用户已登录。
func (Conversation) RequireLoggedIn(userID uint) error {
	if userID == 0 {
		return ErrNotLoggedIn
	}

	return nil
}

// ValidatePeer 校验「当前用户 + 对端用户」参与者规则。
//
// selfErr 用于区分不同入口的文案：建会话为「不能给自己发私信」，
// 其余（消息列表 / 已读 / 删除会话）为「peerId 无效」。
func (Conversation) ValidatePeer(userID, peerID uint, selfErr error) error {
	if userID == 0 {
		return ErrNotLoggedIn
	}

	if peerID == 0 {
		return ErrEmptyPeerID
	}

	if peerID == userID {
		return selfErr
	}

	return nil
}
