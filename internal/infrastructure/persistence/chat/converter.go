package chat

import (
	app "f_ccg_video_back_ddd/internal/application/chat"
	domain "f_ccg_video_back_ddd/internal/domain/chat"
	"f_ccg_video_back_ddd/internal/infrastructure/persistence/mongopo"
)

// toPOMedia 领域媒体信息 → Mongo 文档结构。
func toPOMedia(media *domain.Media) *mongopo.ChatMessageMedia {
	if media == nil {
		return nil
	}

	return &mongopo.ChatMessageMedia{
		URL:    media.URL,
		Mime:   media.Mime,
		Width:  media.Width,
		Height: media.Height,
		Size:   media.Size,
	}
}

// toMediaItem Mongo 文档结构 → 读模型媒体信息。
func toMediaItem(media *mongopo.ChatMessageMedia) *app.MediaItem {
	if media == nil {
		return nil
	}

	return &app.MediaItem{
		URL:    media.URL,
		Mime:   media.Mime,
		Width:  media.Width,
		Height: media.Height,
		Size:   media.Size,
	}
}

// toMessageItem Mongo 消息文档 → 读模型（ObjectID 以十六进制字符串承载）。
func toMessageItem(row *mongopo.ChatMessage) app.MessageItem {
	if row == nil {
		return app.MessageItem{}
	}

	return app.MessageItem{
		ID:             row.ID.Hex(),
		ConversationID: row.ConversationID,
		SenderID:       row.SenderID,
		ReceiverID:     row.ReceiverID,
		Type:           row.Type,
		Text:           row.Text,
		Emoji:          row.Emoji,
		Media:          toMediaItem(row.Media),
		CreatedAt:      row.CreatedAt,
	}
}

// toLastMessageItem Mongo 会话摘要 → 读模型。
func toLastMessageItem(last *mongopo.ChatConversationLastMessage) *app.LastMessageItem {
	if last == nil {
		return nil
	}

	return &app.LastMessageItem{
		MsgID:     last.MsgID.Hex(),
		Type:      last.Type,
		Preview:   last.Preview,
		SenderID:  last.SenderID,
		CreatedAt: last.CreatedAt,
	}
}

// toDomainLastMessage Mongo 会话摘要 → 领域摘要。
func toDomainLastMessage(last *mongopo.ChatConversationLastMessage) *domain.LastMessage {
	if last == nil {
		return nil
	}

	return &domain.LastMessage{
		MsgID:     last.MsgID.Hex(),
		Type:      last.Type,
		Preview:   last.Preview,
		SenderID:  last.SenderID,
		CreatedAt: last.CreatedAt,
	}
}

// toDomainConversation Mongo 会话文档 → 领域聚合。
func toDomainConversation(row *mongopo.ChatConversation) *domain.Conversation {
	if row == nil {
		return nil
	}

	return &domain.Conversation{
		ID:          row.ID,
		MemberA:     toDomainMember(row.MemberA),
		MemberB:     toDomainMember(row.MemberB),
		LastMessage: toDomainLastMessage(row.LastMessage),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func toDomainMember(member mongopo.ChatConversationMember) domain.Member {
	return domain.Member{
		UserID:  member.UserID,
		Unread:  member.Unread,
		Visible: member.Visible,
	}
}
