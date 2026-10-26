package clienterror

import (
	domainchat "f_ccg_video_back_ddd/internal/domain/chat"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// domainchatDescriptions explicitly classifies safe messages in this business context.
var domainchatDescriptions = []Descriptor{
	{Error: domainchat.ErrConversationNotFound, Code: "chat.conversation_not_found", Kind: apperr.NotFound, Message: domainchat.ErrConversationNotFound.Error()},
	{Error: domainchat.ErrDailySendLimit, Code: "chat.daily_send_limit", Kind: apperr.RateLimited, Message: domainchat.ErrDailySendLimit.Error()},
	{Error: domainchat.ErrEmptyEmoji, Code: "chat.empty_emoji", Kind: apperr.Validation, Message: domainchat.ErrEmptyEmoji.Error()},
	{Error: domainchat.ErrEmptyMediaURL, Code: "chat.empty_media_url", Kind: apperr.Validation, Message: domainchat.ErrEmptyMediaURL.Error()},
	{Error: domainchat.ErrEmptyPeerID, Code: "chat.empty_peer_id", Kind: apperr.Validation, Message: domainchat.ErrEmptyPeerID.Error()},
	{Error: domainchat.ErrEmptyText, Code: "chat.empty_text", Kind: apperr.Validation, Message: domainchat.ErrEmptyText.Error()},
	{Error: domainchat.ErrInvalidCursor, Code: "chat.invalid_cursor", Kind: apperr.Validation, Message: domainchat.ErrInvalidCursor.Error()},
	{Error: domainchat.ErrInvalidPeerID, Code: "chat.invalid_peer_id", Kind: apperr.Validation, Message: domainchat.ErrInvalidPeerID.Error()},
	{Error: domainchat.ErrNotLoggedIn, Code: "chat.not_logged_in", Kind: apperr.Rejected, Message: domainchat.ErrNotLoggedIn.Error()},
	{Error: domainchat.ErrSelfConversation, Code: "chat.self_conversation", Kind: apperr.Rejected, Message: domainchat.ErrSelfConversation.Error()},
	{Error: domainchat.ErrSelfMessage, Code: "chat.self_message", Kind: apperr.Rejected, Message: domainchat.ErrSelfMessage.Error()},
	{Error: domainchat.ErrStoreUnavailable, Code: "chat.store_unavailable", Kind: apperr.Dependency, Message: domainchat.ErrStoreUnavailable.Error()},
	{Error: domainchat.ErrTextTooLong, Code: "chat.text_too_long", Kind: apperr.Validation, Message: domainchat.ErrTextTooLong.Error()},
	{Error: domainchat.ErrUnsupportedMessageType, Code: "chat.unsupported_message_type", Kind: apperr.Validation, Message: domainchat.ErrUnsupportedMessageType.Error()},
}
