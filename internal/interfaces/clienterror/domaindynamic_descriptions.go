package clienterror

import (
	domaindynamic "f_ccg_video_back_ddd/internal/domain/dynamic"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// domaindynamicDescriptions explicitly classifies safe messages in this business context.
var domaindynamicDescriptions = []Descriptor{
	{Error: domaindynamic.ErrAuthorIDRequired, Code: "dynamic.author_idrequired", Kind: apperr.Validation, Message: domaindynamic.ErrAuthorIDRequired.Error()},
	{Error: domaindynamic.ErrContentEmpty, Code: "dynamic.content_empty", Kind: apperr.Validation, Message: domaindynamic.ErrContentEmpty.Error()},
	{Error: domaindynamic.ErrContentTooLong, Code: "dynamic.content_too_long", Kind: apperr.Validation, Message: domaindynamic.ErrContentTooLong.Error()},
	{Error: domaindynamic.ErrDeleteForbidden, Code: "dynamic.delete_forbidden", Kind: apperr.Rejected, Message: domaindynamic.ErrDeleteForbidden.Error()},
	{Error: domaindynamic.ErrDynamicIDRequired, Code: "dynamic.dynamic_idrequired", Kind: apperr.Validation, Message: domaindynamic.ErrDynamicIDRequired.Error()},
	{Error: domaindynamic.ErrDynamicIDsRequired, Code: "dynamic.dynamic_ids_required", Kind: apperr.Validation, Message: domaindynamic.ErrDynamicIDsRequired.Error()},
	{Error: domaindynamic.ErrDynamicNotFound, Code: "dynamic.dynamic_not_found", Kind: apperr.NotFound, Message: domaindynamic.ErrDynamicNotFound.Error()},
	{Error: domaindynamic.ErrLikeDuplicated, Code: "dynamic.like_duplicated", Kind: apperr.Conflict, Message: domaindynamic.ErrLikeDuplicated.Error()},
	{Error: domaindynamic.ErrLoginRequired, Code: "dynamic.login_required", Kind: apperr.Validation, Message: domaindynamic.ErrLoginRequired.Error()},
	{Error: domaindynamic.ErrPinForbidden, Code: "dynamic.pin_forbidden", Kind: apperr.Rejected, Message: domaindynamic.ErrPinForbidden.Error()},
	{Error: domaindynamic.ErrRecordNotFound, Code: "dynamic.record_not_found", Kind: apperr.NotFound, Message: domaindynamic.ErrRecordNotFound.Error()},
	{Error: domaindynamic.ErrUserIDRequired, Code: "dynamic.user_idrequired", Kind: apperr.Validation, Message: domaindynamic.ErrUserIDRequired.Error()},
	{Error: domaindynamic.ErrUserNotFound, Code: "dynamic.user_not_found", Kind: apperr.NotFound, Message: domaindynamic.ErrUserNotFound.Error()},
	{Error: domaindynamic.ErrUserNotLogin, Code: "dynamic.user_not_login", Kind: apperr.Rejected, Message: domaindynamic.ErrUserNotLogin.Error()},
	{Error: domaindynamic.ErrVideoIDRequired, Code: "dynamic.video_idrequired", Kind: apperr.Validation, Message: domaindynamic.ErrVideoIDRequired.Error()},
	{Error: domaindynamic.ErrVideoPinForbidden, Code: "dynamic.video_pin_forbidden", Kind: apperr.Rejected, Message: domaindynamic.ErrVideoPinForbidden.Error()},
}
