package clienterror

import (
	applicationvideo "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// applicationvideoDescriptions explicitly classifies safe messages in this business context.
var applicationvideoDescriptions = []Descriptor{
	{Error: applicationvideo.ErrDailyUploadLimit, Code: "video.app_daily_upload_limit", Kind: apperr.RateLimited, Message: applicationvideo.ErrDailyUploadLimit.Error()},
	{Error: applicationvideo.ErrUploadQuotaUnavailable, Code: "video.app_upload_quota_unavailable", Kind: apperr.Dependency, Message: applicationvideo.ErrUploadQuotaUnavailable.Error()},
	{Error: applicationvideo.ErrUploadStorageUnavailable, Code: "video.app_upload_storage_unavailable", Kind: apperr.Dependency, Message: applicationvideo.ErrUploadStorageUnavailable.Error()},
	{Error: applicationvideo.ErrVectorRebuildInProgress, Code: "video.app_vector_rebuild_in_progress", Kind: apperr.Conflict, Message: applicationvideo.ErrVectorRebuildInProgress.Error()},
}
