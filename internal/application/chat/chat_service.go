package chat

import (
	"context"
	"time"

	domain "f_ccg_video_back_ddd/internal/domain/chat"

	"go.uber.org/zap"
)

// Service 私信用例服务（原 service/chat_service 的 ChatService）。
//
// 原实现是无字段的 `ChatService struct{}`，直读 global.MongoDB / global.DB / global.RedisClient；
// 新架构改为构造函数注入写侧仓储、读侧查询端口、关注关系查询与通知角标失效端口。
type Service struct {
	repo    domain.Repository
	query   Query
	follows FollowRelationQuery
	badges  NotificationBadgeInvalidator
	logger  *zap.Logger
}

// NewService 构造私信用例服务。
func NewService(
	repo domain.Repository,
	query Query,
	follows FollowRelationQuery,
	badges NotificationBadgeInvalidator,
	logger *zap.Logger,
) *Service {
	return &Service{
		repo:    repo,
		query:   query,
		follows: follows,
		badges:  badges,
		logger:  logger,
	}
}

// SendMessage 校验并落库一条私信，同时推进会话摘要与接收方未读数。
//
// InsertMessage 与 ApplySend 当前是连续两次仓储调用，没有共同的 Mongo 事务。
// 后一步失败时消息可能已经存在；错误不代表此前写入均被撤销，调用方重试也不能假定自动去重。
func (s *Service) SendMessage(
	ctx context.Context,
	senderID, receiverID uint,
	msgType, text, emoji string,
	media *MediaItem,
) (*MessageItem, error) {
	if !s.repo.Available() {
		return nil, domain.ErrStoreUnavailable
	}

	if err := domain.ValidateSender(senderID, receiverID); err != nil {
		return nil, err
	}

	domainMedia := toDomainMedia(media)
	if err := domain.ValidateContent(msgType, text, emoji, domainMedia); err != nil {
		return nil, err
	}

	conversationID := domain.BuildConversationID(senderID, receiverID)

	now := nowTime()
	if err := s.checkDailySendLimit(ctx, senderID, receiverID, conversationID, now); err != nil {
		return nil, err
	}

	msg := &domain.Message{
		ConversationID: conversationID,
		SenderID:       senderID,
		ReceiverID:     receiverID,
		Type:           msgType,
		Text:           text,
		Emoji:          emoji,
		Media:          domainMedia,
		CreatedAt:      now,
	}

	if err := s.repo.InsertMessage(ctx, msg); err != nil {
		return nil, err
	}

	if err := s.repo.ApplySend(ctx, msg, domain.BuildPreview(msg)); err != nil {
		return nil, err
	}

	// 聊天未读变化后失效通知角标缓存（跨上下文经端口，不 import notification 包）。
	s.invalidateBadge(msg.ReceiverID)

	item := toMessageItem(msg)

	return &item, nil
}

// ListConversations 会话列表分页（pageSize 由领域规则二次 clamp 到 50）。
func (s *Service) ListConversations(
	ctx context.Context,
	userID uint,
	page, pageSize int,
) ([]ConversationItem, int64, error) {
	if !s.repo.Available() {
		return nil, 0, domain.ErrStoreUnavailable
	}

	if userID == 0 {
		return nil, 0, domain.ErrNotLoggedIn
	}

	page, pageSize = domain.NormalizeConversationPage(page, pageSize)

	return s.query.ListConversations(ctx, userID, page, pageSize)
}

// ListMessages 按 before 游标向前翻页拉取会话消息（固定每页 10 条）。
func (s *Service) ListMessages(ctx context.Context, userID, peerID uint, before string) (*MessageListResult, error) {
	if !s.repo.Available() {
		return nil, domain.ErrStoreUnavailable
	}

	if err := domain.ValidatePeer(userID, peerID, domain.ErrInvalidPeerID); err != nil {
		return nil, err
	}

	if err := domain.ValidateMessageCursor(before); err != nil {
		return nil, err
	}

	rows, err := s.query.ListMessagesBefore(ctx, userID, peerID, before, domain.MessageFetchLimit)
	if err != nil {
		return nil, err
	}

	list, hasMore := domain.TrimMessagePage(rows)

	res := &MessageListResult{List: list, HasMore: hasMore}
	if hasMore && len(list) > 0 {
		res.NextBefore = list[0].ID
	}

	return res, nil
}

// MarkRead 标记与某用户的私信已读（未读清零 + 联动通知角标）。
func (s *Service) MarkRead(ctx context.Context, userID, peerID uint) error {
	if !s.repo.Available() {
		return domain.ErrStoreUnavailable
	}

	if err := domain.ValidatePeer(userID, peerID, domain.ErrInvalidPeerID); err != nil {
		return err
	}

	matched, err := s.repo.MarkRead(ctx, userID, peerID, nowTime())
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
		return 0, domain.ErrStoreUnavailable
	}

	if userID == 0 {
		return 0, domain.ErrNotLoggedIn
	}

	return s.query.TotalUnread(ctx, userID)
}

// CreateConversation 创建会话（幂等）。若会话不存在会创建一个空会话（无 lastMessage）。
func (s *Service) CreateConversation(ctx context.Context, userID, peerID uint) (*ConversationItem, error) {
	if !s.repo.Available() {
		return nil, domain.ErrStoreUnavailable
	}

	if err := domain.ValidatePeer(userID, peerID, domain.ErrSelfConversation); err != nil {
		return nil, err
	}

	conv, err := s.repo.UpsertConversation(ctx, userID, peerID, nowTime())
	if err != nil {
		return nil, err
	}

	peer, unread := conv.PeerAndUnread(userID)

	return &ConversationItem{
		ID:          conv.ID,
		PeerID:      peer,
		Unread:      unread,
		LastMessage: toLastMessageItem(conv.LastMessage),
		UpdatedAt:   conv.DisplayTime().Unix(),
	}, nil
}

// DeleteConversation 仅对当前用户隐藏会话，保留对端会话和历史消息。
func (s *Service) DeleteConversation(ctx context.Context, userID, peerID uint) error {
	if !s.repo.Available() {
		return domain.ErrStoreUnavailable
	}

	if err := domain.ValidatePeer(userID, peerID, domain.ErrInvalidPeerID); err != nil {
		return err
	}

	return s.repo.HideConversation(ctx, userID, peerID, nowTime())
}

// checkDailySendLimit 对端未关注自己时限制每日仅可发送一条，直到对端回复。
//
// 与原实现一致：关注关系查询失败仅告警并放行，不因此拒绝发送。
func (s *Service) checkDailySendLimit(
	ctx context.Context,
	senderID, receiverID uint,
	conversationID string,
	now time.Time,
) error {
	if !domain.NeedsDailySendLimitCheck(senderID, receiverID) {
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

	dayStart, dayEnd := domain.DayRange(now)

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

	return domain.EvaluateDailySendLimit(sentToday, repliedToday)
}

// invalidateBadge 失效接收者的通知角标缓存（端口缺失时静默跳过）。
func (s *Service) invalidateBadge(userID uint) {
	if s.badges == nil || userID == 0 {
		return
	}

	s.badges.InvalidateCount(int(userID))
}

func nowTime() time.Time {
	return time.Now()
}

func toDomainMedia(media *MediaItem) *domain.Media {
	if media == nil {
		return nil
	}

	return &domain.Media{
		URL:    media.URL,
		Mime:   media.Mime,
		Width:  media.Width,
		Height: media.Height,
		Size:   media.Size,
	}
}

func toMediaItem(media *domain.Media) *MediaItem {
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

func toMessageItem(msg *domain.Message) MessageItem {
	if msg == nil {
		return MessageItem{}
	}

	return MessageItem{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		SenderID:       msg.SenderID,
		ReceiverID:     msg.ReceiverID,
		Type:           msg.Type,
		Text:           msg.Text,
		Emoji:          msg.Emoji,
		Media:          toMediaItem(msg.Media),
		CreatedAt:      msg.CreatedAt,
	}
}

func toLastMessageItem(last *domain.LastMessage) *LastMessageItem {
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
