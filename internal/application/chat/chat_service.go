package chat

import (
	"context"
	"time"

	chatdomain "f_ccg_video_back_ddd/internal/domain/chat"

	"go.uber.org/zap"
)

// Service 编排私信用例。
//
// 写侧仓储、读侧查询、关注关系查询和通知角标失效能力通过构造函数注入。
type Service struct {
	repo      chatdomain.Repository
	query     Query
	follows   FollowRelationQuery
	badges    NotificationBadgeInvalidator
	logger    *zap.Logger
	sendLimit chatdomain.SendLimit
	pages     chatdomain.PagePolicy[MessageItem]
}

// NewService 构造私信用例服务。
func NewService(
	repo chatdomain.Repository,
	query Query,
	follows FollowRelationQuery,
	badges NotificationBadgeInvalidator,
	logger *zap.Logger,
) *Service {
	return &Service{
		repo:      repo,
		query:     query,
		follows:   follows,
		badges:    badges,
		logger:    logger,
		sendLimit: chatdomain.SendLimit{},
		pages:     chatdomain.PagePolicy[MessageItem]{},
	}
}

// SendMessage 校验并落库一条私信，同时推进会话摘要与接收方未读数。
//
// The repository commits the message and conversation summary in one Mongo transaction.
func (s *Service) SendMessage(
	ctx context.Context,
	senderID, receiverID uint,
	msgType, text, emoji string,
	media *MediaItem,
) (*MessageItem, error) {
	if !s.repo.Available() {
		return nil, chatdomain.ErrStoreUnavailable
	}

	now := s.now()
	msg, err := chatdomain.NewMessage(senderID, receiverID, msgType, text, emoji, s.toDomainMedia(media), now)
	if err != nil {
		return nil, err
	}

	if err := s.checkDailySendLimit(ctx, senderID, receiverID, msg.ConversationID(), now); err != nil {
		return nil, err
	}

	stored, err := s.repo.SendMessage(ctx, msg, msg.Preview())
	if err != nil {
		return nil, err
	}

	// 聊天未读变化后失效通知角标缓存（跨上下文经端口，不 import notification 包）。
	s.invalidateBadge(stored.ReceiverID())

	item := s.toMessageItem(stored)

	return &item, nil
}

// ListConversations 会话列表分页（pageSize 由领域规则二次 clamp 到 50）。
func (s *Service) ListConversations(
	ctx context.Context,
	userID uint,
	page, pageSize int,
) ([]ConversationItem, int64, error) {
	if !s.repo.Available() {
		return nil, 0, chatdomain.ErrStoreUnavailable
	}

	if err := (chatdomain.Conversation{}).RequireLoggedIn(userID); err != nil {
		return nil, 0, err
	}

	page, pageSize = s.pages.NormalizeConversationPage(page, pageSize)

	return s.query.ListConversations(ctx, userID, page, pageSize)
}

// ListMessages 按 before 游标向前翻页拉取会话消息（固定每页 10 条）。
func (s *Service) ListMessages(ctx context.Context, userID, peerID uint, before string) (*MessageListResult, error) {
	if !s.repo.Available() {
		return nil, chatdomain.ErrStoreUnavailable
	}

	if err := (chatdomain.Conversation{}).ValidatePeer(userID, peerID, chatdomain.ErrInvalidPeerID); err != nil {
		return nil, err
	}

	if err := s.pages.ValidateMessageCursor(before); err != nil {
		return nil, err
	}

	rows, err := s.query.ListMessagesBefore(ctx, userID, peerID, before, chatdomain.MessageFetchLimit)
	if err != nil {
		return nil, err
	}

	list, hasMore := s.pages.TrimMessagePage(rows)

	res := &MessageListResult{List: list, HasMore: hasMore}
	if hasMore && len(list) > 0 {
		res.NextBefore = list[0].ID
	}

	return res, nil
}

// MarkRead 标记与某用户的私信已读（未读清零 + 联动通知角标）。
func (s *Service) MarkRead(ctx context.Context, userID, peerID uint) error {
	if !s.repo.Available() {
		return chatdomain.ErrStoreUnavailable
	}

	if err := (chatdomain.Conversation{}).ValidatePeer(userID, peerID, chatdomain.ErrInvalidPeerID); err != nil {
		return err
	}

	matched, err := s.repo.MarkRead(ctx, userID, peerID, s.now())
	if err != nil {
		return err
	}

	if !matched {
		return nil
	}

	// 已读清零后联动通知角标（message 维度）。
	s.invalidateBadge(userID)

	return nil
}

// TotalUnread 汇总用户所有可见会话的未读总数。
//
// 该方法同时满足 notification 上下文声明的 ChatUnreadCounter 端口。
func (s *Service) TotalUnread(ctx context.Context, userID uint) (int64, error) {
	if !s.repo.Available() {
		return 0, chatdomain.ErrStoreUnavailable
	}

	if err := (chatdomain.Conversation{}).RequireLoggedIn(userID); err != nil {
		return 0, err
	}

	return s.query.TotalUnread(ctx, userID)
}

// CreateConversation 创建会话（幂等）。若会话不存在会创建一个空会话（无 lastMessage）。
func (s *Service) CreateConversation(ctx context.Context, userID, peerID uint) (*ConversationItem, error) {
	if !s.repo.Available() {
		return nil, chatdomain.ErrStoreUnavailable
	}

	if err := (chatdomain.Conversation{}).ValidatePeer(userID, peerID, chatdomain.ErrSelfConversation); err != nil {
		return nil, err
	}

	conv, err := s.repo.UpsertConversation(ctx, userID, peerID, s.now())
	if err != nil {
		return nil, err
	}

	peer, unread := conv.PeerAndUnread(userID)

	return &ConversationItem{
		ID:          conv.ID(),
		PeerID:      peer,
		Unread:      unread,
		LastMessage: s.toLastMessageItem(conv.LastMessage()),
		UpdatedAt:   conv.DisplayTime().Unix(),
	}, nil
}

// DeleteConversation 仅对当前用户隐藏会话，保留对端会话和历史消息。
func (s *Service) DeleteConversation(ctx context.Context, userID, peerID uint) error {
	if !s.repo.Available() {
		return chatdomain.ErrStoreUnavailable
	}

	if err := (chatdomain.Conversation{}).ValidatePeer(userID, peerID, chatdomain.ErrInvalidPeerID); err != nil {
		return err
	}

	return s.repo.HideConversation(ctx, userID, peerID, s.now())
}

// checkDailySendLimit 对端未关注自己时限制每日仅可发送一条，直到对端回复。
//
// 关注关系查询失败仅告警并放行，不因此拒绝发送。
func (s *Service) checkDailySendLimit(
	ctx context.Context,
	senderID, receiverID uint,
	conversationID string,
	now time.Time,
) error {
	if !s.sendLimit.NeedsDailySendLimitCheck(senderID, receiverID) {
		return nil
	}

	if ctx == nil {
		ctx = context.TODO()
	}

	receiverFollowsSender, err := s.follows.IsFollowing(ctx, receiverID, senderID)
	if err != nil {
		s.logger.Warn("check focus relation failed, skip chat daily limit",
			zap.Error(err),
			zap.Uint("senderId", senderID),
			zap.Uint("receiverId", receiverID),
		)

		return nil
	}

	if receiverFollowsSender {
		return nil
	}

	dayStart, dayEnd := s.sendLimit.DayRange(now)

	sentToday, err := s.repo.ExistsMessageBetween(ctx, conversationID, senderID, receiverID, dayStart, dayEnd)
	if err != nil {
		return err
	}

	if !sentToday {
		return nil
	}

	repliedToday, err := s.repo.ExistsMessageBetween(ctx, conversationID, receiverID, senderID, dayStart, dayEnd)
	if err != nil {
		return err
	}

	return s.sendLimit.EvaluateDailySendLimit(sentToday, repliedToday)
}

// invalidateBadge 失效接收者的通知角标缓存（端口缺失时静默跳过）。
func (s *Service) invalidateBadge(userID uint) {
	if s.badges == nil || userID == 0 {
		return
	}

	s.badges.InvalidateCount(int(userID))
}

func (s *Service) now() time.Time {
	return time.Now()
}

func (s *Service) toDomainMedia(media *MediaItem) *chatdomain.Media {
	if media == nil {
		return nil
	}

	return &chatdomain.Media{
		URL:    media.URL,
		Mime:   media.Mime,
		Width:  media.Width,
		Height: media.Height,
		Size:   media.Size,
	}
}

func (s *Service) toMediaItem(media *chatdomain.Media) *MediaItem {
	if media == nil {
		return nil
	}

	return &MediaItem{
		URL:    media.URL,
		Mime:   media.Mime,
		Width:  media.Width,
		Height: media.Height,
		Size:   media.Size,
	}
}

func (s *Service) toMessageItem(msg *chatdomain.Message) MessageItem {
	if msg == nil {
		return MessageItem{}
	}

	return MessageItem{
		ID:             msg.ID(),
		ConversationID: msg.ConversationID(),
		SenderID:       msg.SenderID(),
		ReceiverID:     msg.ReceiverID(),
		Type:           msg.Type(),
		Text:           msg.Text(),
		Emoji:          msg.Emoji(),
		Media:          s.toMediaItem(msg.Media()),
		CreatedAt:      msg.CreatedAt(),
	}
}

func (s *Service) toLastMessageItem(last *chatdomain.LastMessage) *LastMessageItem {
	if last == nil {
		return nil
	}

	return &LastMessageItem{
		MsgID:     last.MsgID,
		Type:      last.Type,
		Preview:   last.Preview,
		SenderID:  last.SenderID,
		CreatedAt: last.CreatedAt,
	}
}
