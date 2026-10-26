package clienterror

import (
	domainnotification "f_ccg_video_back_ddd/internal/domain/notification"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// domainnotificationDescriptions explicitly classifies safe messages in this business context.
var domainnotificationDescriptions = []Descriptor{
	{Error: domainnotification.ErrSystemNotificationNotDeletable, Code: "notification.system_notification_not_deletable", Kind: apperr.Rejected, Message: domainnotification.ErrSystemNotificationNotDeletable.Error()},
}
