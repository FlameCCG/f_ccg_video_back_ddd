package clienterror

import (
	domaincomment "f_ccg_video_back_ddd/internal/domain/comment"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// domaincommentDescriptions explicitly classifies safe messages in this business context.
var domaincommentDescriptions = []Descriptor{
	{Error: domaincomment.ErrAuthorIDRequired, Code: "comment.author_idrequired", Kind: apperr.Validation, Message: domaincomment.ErrAuthorIDRequired.Error()},
	{Error: domaincomment.ErrAuthorNotFound, Code: "comment.author_not_found", Kind: apperr.NotFound, Message: domaincomment.ErrAuthorNotFound.Error()},
	{Error: domaincomment.ErrCommentIDRequired, Code: "comment.comment_idrequired", Kind: apperr.Validation, Message: domaincomment.ErrCommentIDRequired.Error()},
	{Error: domaincomment.ErrCommentIDsRequired, Code: "comment.comment_ids_required", Kind: apperr.Validation, Message: domaincomment.ErrCommentIDsRequired.Error()},
	{Error: domaincomment.ErrCommentNotFound, Code: "comment.comment_not_found", Kind: apperr.NotFound, Message: domaincomment.ErrCommentNotFound.Error()},
	{Error: domaincomment.ErrContentEmpty, Code: "comment.content_empty", Kind: apperr.Validation, Message: domaincomment.ErrContentEmpty.Error()},
	{Error: domaincomment.ErrContentTooLong, Code: "comment.content_too_long", Kind: apperr.Validation, Message: domaincomment.ErrContentTooLong.Error()},
	{Error: domaincomment.ErrDeleteForbidden, Code: "comment.delete_forbidden", Kind: apperr.Rejected, Message: domaincomment.ErrDeleteForbidden.Error()},
	{Error: domaincomment.ErrDynamicNotFound, Code: "comment.dynamic_not_found", Kind: apperr.NotFound, Message: domaincomment.ErrDynamicNotFound.Error()},
	{Error: domaincomment.ErrLikeCacheUnavailable, Code: "comment.like_cache_unavailable", Kind: apperr.Dependency, Message: domaincomment.ErrLikeCacheUnavailable.Error()},
	{Error: domaincomment.ErrLikeDuplicated, Code: "comment.like_duplicated", Kind: apperr.Conflict, Message: domaincomment.ErrLikeDuplicated.Error()},
	{Error: domaincomment.ErrParentNotFound, Code: "comment.parent_not_found", Kind: apperr.NotFound, Message: domaincomment.ErrParentNotFound.Error()},
	{Error: domaincomment.ErrPinForbidden, Code: "comment.pin_forbidden", Kind: apperr.Rejected, Message: domaincomment.ErrPinForbidden.Error()},
	{Error: domaincomment.ErrPinRootOnly, Code: "comment.pin_root_only", Kind: apperr.Rejected, Message: domaincomment.ErrPinRootOnly.Error()},
	{Error: domaincomment.ErrPinTargetMissing, Code: "comment.pin_target_missing", Kind: apperr.Rejected, Message: domaincomment.ErrPinTargetMissing.Error()},
	{Error: domaincomment.ErrRecordNotFound, Code: "comment.record_not_found", Kind: apperr.NotFound, Message: domaincomment.ErrRecordNotFound.Error()},
	{Error: domaincomment.ErrTargetRequired, Code: "comment.target_required", Kind: apperr.Validation, Message: domaincomment.ErrTargetRequired.Error()},
	{Error: domaincomment.ErrUserNotLogin, Code: "comment.user_not_login", Kind: apperr.Rejected, Message: domaincomment.ErrUserNotLogin.Error()},
	{Error: domaincomment.ErrVideoNotPublished, Code: "comment.video_not_published", Kind: apperr.Rejected, Message: domaincomment.ErrVideoNotPublished.Error()},
}
