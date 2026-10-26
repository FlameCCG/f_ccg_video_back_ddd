package clienterror

import (
	applicationnotification "f_ccg_video_back_ddd/internal/application/notification"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// applicationnotificationDescriptions explicitly classifies safe messages in this business context.
var applicationnotificationDescriptions = []Descriptor{
	{Error: applicationnotification.ErrNoticeNotFound, Code: "notification.app_notice_not_found", Kind: apperr.NotFound, Message: applicationnotification.ErrNoticeNotFound.Error()},
	{Error: applicationnotification.ErrNoticeQueryFailed, Code: "notification.app_notice_query_failed", Kind: apperr.Internal, Message: applicationnotification.ErrNoticeQueryFailed.Error()},
	{Error: applicationnotification.ErrNoticeUpdateFailed, Code: "notification.app_notice_update_failed", Kind: apperr.Internal, Message: applicationnotification.ErrNoticeUpdateFailed.Error()},
}
