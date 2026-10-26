//nolint:dupl // The logging contract keeps safe descriptors owned by each business context.
package clienterror

import (
	applicationcontentops "f_ccg_video_back_ddd/internal/application/contentops"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// applicationcontentopsDescriptions explicitly classifies safe messages in this business context.
var applicationcontentopsDescriptions = []Descriptor{
	{Error: applicationcontentops.ErrBannerPartitionFilterUnsupported, Code: "contentops.app_banner_partition_filter_unsupported", Kind: apperr.Validation, Message: applicationcontentops.ErrBannerPartitionFilterUnsupported.Error()},
	{Error: applicationcontentops.ErrConfigFileRead, Code: "contentops.app_config_file_read", Kind: apperr.Internal, Message: applicationcontentops.ErrConfigFileRead.Error()},
	{Error: applicationcontentops.ErrConfigFileWrite, Code: "contentops.app_config_file_write", Kind: apperr.Internal, Message: applicationcontentops.ErrConfigFileWrite.Error()},
	{Error: applicationcontentops.ErrConfigItemUnsupported, Code: "contentops.app_config_item_unsupported", Kind: apperr.Validation, Message: applicationcontentops.ErrConfigItemUnsupported.Error()},
	{Error: applicationcontentops.ErrSiteConfigNotInitialized, Code: "contentops.app_site_config_not_initialized", Kind: apperr.Internal, Message: applicationcontentops.ErrSiteConfigNotInitialized.Error()},
}
