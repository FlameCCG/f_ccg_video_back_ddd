//nolint:dupl // The logging contract keeps safe descriptors owned by each business context.
package clienterror

import (
	domaincontentops "f_ccg_video_back_ddd/internal/domain/contentops"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// domaincontentopsDescriptions explicitly classifies safe messages in this business context.
var domaincontentopsDescriptions = []Descriptor{
	{Error: domaincontentops.ErrBannerInvalidParam, Code: "contentops.banner_invalid_param", Kind: apperr.Validation, Message: domaincontentops.ErrBannerInvalidParam.Error()},
	{Error: domaincontentops.ErrBannerNoUpdatableField, Code: "contentops.banner_no_updatable_field", Kind: apperr.Rejected, Message: domaincontentops.ErrBannerNoUpdatableField.Error()},
	{Error: domaincontentops.ErrBannerNotFound, Code: "contentops.banner_not_found", Kind: apperr.NotFound, Message: domaincontentops.ErrBannerNotFound.Error()},
	{Error: domaincontentops.ErrBannerPartitionNotFound, Code: "contentops.banner_partition_not_found", Kind: apperr.NotFound, Message: domaincontentops.ErrBannerPartitionNotFound.Error()},
	{Error: domaincontentops.ErrBannerPartitionTypeUnsupported, Code: "contentops.banner_partition_type_unsupported", Kind: apperr.Validation, Message: domaincontentops.ErrBannerPartitionTypeUnsupported.Error()},
}
