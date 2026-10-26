package clienterror

import (
	domainmedia "f_ccg_video_back_ddd/internal/domain/media"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// domainmediaDescriptions explicitly classifies safe messages in this business context.
var domainmediaDescriptions = []Descriptor{
	{Error: domainmedia.ErrTranscodeEncodeTimeout, Code: "media.transcode_encode_timeout", Kind: apperr.Dependency, Message: domainmedia.ErrTranscodeEncodeTimeout.Error()},
	{Error: domainmedia.ErrTranscodeProgressStalled, Code: "media.transcode_progress_stalled", Kind: apperr.Dependency, Message: domainmedia.ErrTranscodeProgressStalled.Error()},
	{Error: domainmedia.ErrTranscodeUploadTimeout, Code: "media.transcode_upload_timeout", Kind: apperr.Dependency, Message: domainmedia.ErrTranscodeUploadTimeout.Error()},
	{Error: domainmedia.ErrVideoDeleted, Code: "media.video_deleted", Kind: apperr.Rejected, Message: domainmedia.ErrVideoDeleted.Error()},
}
