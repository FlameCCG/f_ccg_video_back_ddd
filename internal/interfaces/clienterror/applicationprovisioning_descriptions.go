package clienterror

import (
	applicationprovisioning "f_ccg_video_back_ddd/internal/application/provisioning"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// applicationprovisioningDescriptions explicitly classifies safe messages in this business context.
var applicationprovisioningDescriptions = []Descriptor{
	{Error: applicationprovisioning.ErrBannerImagesDirUnreadable, Code: "provisioning.app_banner_images_dir_unreadable", Kind: apperr.Rejected, Message: applicationprovisioning.ErrBannerImagesDirUnreadable.Error()},
	{Error: applicationprovisioning.ErrSuperUserArgsRequired, Code: "provisioning.app_super_user_args_required", Kind: apperr.Validation, Message: applicationprovisioning.ErrSuperUserArgsRequired.Error()},
}
