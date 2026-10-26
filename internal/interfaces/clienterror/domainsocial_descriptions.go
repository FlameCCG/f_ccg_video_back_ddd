package clienterror

import (
	domainsocial "f_ccg_video_back_ddd/internal/domain/social"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// domainsocialDescriptions explicitly classifies safe messages in this business context.
var domainsocialDescriptions = []Descriptor{
	{Error: domainsocial.ErrAlreadyFocused, Code: "social.already_focused", Kind: apperr.Conflict, Message: domainsocial.ErrAlreadyFocused.Error()},
	{Error: domainsocial.ErrCannotFocusSelf, Code: "social.cannot_focus_self", Kind: apperr.Rejected, Message: domainsocial.ErrCannotFocusSelf.Error()},
	{Error: domainsocial.ErrFansNotPublic, Code: "social.fans_not_public", Kind: apperr.Rejected, Message: domainsocial.ErrFansNotPublic.Error()},
	{Error: domainsocial.ErrFocusFailed, Code: "social.focus_failed", Kind: apperr.Internal, Message: domainsocial.ErrFocusFailed.Error()},
	{Error: domainsocial.ErrFollowNotPublic, Code: "social.follow_not_public", Kind: apperr.Rejected, Message: domainsocial.ErrFollowNotPublic.Error()},
	{Error: domainsocial.ErrForeignPageLimited, Code: "social.foreign_page_limited", Kind: apperr.Rejected, Message: domainsocial.ErrForeignPageLimited.Error()},
	{Error: domainsocial.ErrListQueryFailed, Code: "social.list_query_failed", Kind: apperr.Internal, Message: domainsocial.ErrListQueryFailed.Error()},
	{Error: domainsocial.ErrNotFocused, Code: "social.not_focused", Kind: apperr.Rejected, Message: domainsocial.ErrNotFocused.Error()},
	{Error: domainsocial.ErrSelfRelation, Code: "social.self_relation", Kind: apperr.Rejected, Message: domainsocial.ErrSelfRelation.Error()},
	{Error: domainsocial.ErrTargetUserNotFound, Code: "social.target_user_not_found", Kind: apperr.NotFound, Message: domainsocial.ErrTargetUserNotFound.Error()},
	{Error: domainsocial.ErrUnfocusFailed, Code: "social.unfocus_failed", Kind: apperr.Internal, Message: domainsocial.ErrUnfocusFailed.Error()},
	{Error: domainsocial.ErrUserIDRequired, Code: "social.user_idrequired", Kind: apperr.Validation, Message: domainsocial.ErrUserIDRequired.Error()},
	{Error: domainsocial.ErrUserNotFound, Code: "social.user_not_found", Kind: apperr.NotFound, Message: domainsocial.ErrUserNotFound.Error()},
}
