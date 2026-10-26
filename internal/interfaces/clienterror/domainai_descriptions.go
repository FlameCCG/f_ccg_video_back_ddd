package clienterror

import (
	domainai "f_ccg_video_back_ddd/internal/domain/ai"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// domainaiDescriptions explicitly classifies safe messages in this business context.
var domainaiDescriptions = []Descriptor{
	{Error: domainai.ErrAssetHostNotAllowed, Code: "ai.asset_host_not_allowed", Kind: apperr.Rejected, Message: domainai.ErrAssetHostNotAllowed.Error()},
	{Error: domainai.ErrChatAPIKeyMissing, Code: "ai.chat_apikey_missing", Kind: apperr.Internal, Message: domainai.ErrChatAPIKeyMissing.Error()},
	{Error: domainai.ErrConfigMissing, Code: "ai.config_missing", Kind: apperr.Internal, Message: domainai.ErrConfigMissing.Error()},
	{Error: domainai.ErrDoubaoAPIKeyMissing, Code: "ai.doubao_apikey_missing", Kind: apperr.Internal, Message: domainai.ErrDoubaoAPIKeyMissing.Error()},
	{Error: domainai.ErrEmptyInput, Code: "ai.empty_input", Kind: apperr.Validation, Message: domainai.ErrEmptyInput.Error()},
	{Error: domainai.ErrEmptyPrompt, Code: "ai.empty_prompt", Kind: apperr.Validation, Message: domainai.ErrEmptyPrompt.Error()},
	{Error: domainai.ErrEmptyRequestID, Code: "ai.empty_request_id", Kind: apperr.Validation, Message: domainai.ErrEmptyRequestID.Error()},
	{Error: domainai.ErrImageCountRange, Code: "ai.image_count_range", Kind: apperr.Validation, Message: domainai.ErrImageCountRange.Error()},
	{Error: domainai.ErrImageRefExclusive, Code: "ai.image_ref_exclusive", Kind: apperr.Validation, Message: domainai.ErrImageRefExclusive.Error()},
	{Error: domainai.ErrImageRefPlusCount, Code: "ai.image_ref_plus_count", Kind: apperr.Validation, Message: domainai.ErrImageRefPlusCount.Error()},
	{Error: domainai.ErrNoUserMessage, Code: "ai.no_user_message", Kind: apperr.Rejected, Message: domainai.ErrNoUserMessage.Error()},
	{Error: domainai.ErrOutputTextNotFound, Code: "ai.output_text_not_found", Kind: apperr.Dependency, Message: domainai.ErrOutputTextNotFound.Error()},
	{Error: domainai.ErrTooManyImageRefs, Code: "ai.too_many_image_refs", Kind: apperr.Rejected, Message: domainai.ErrTooManyImageRefs.Error()},
	{Error: domainai.ErrTooManyVideoRefs, Code: "ai.too_many_video_refs", Kind: apperr.Rejected, Message: domainai.ErrTooManyVideoRefs.Error()},
	{Error: domainai.ErrUnsupportedAssetScheme, Code: "ai.unsupported_asset_scheme", Kind: apperr.Validation, Message: domainai.ErrUnsupportedAssetScheme.Error()},
	{Error: domainai.ErrVideoInputRequired, Code: "ai.video_input_required", Kind: apperr.Validation, Message: domainai.ErrVideoInputRequired.Error()},
	{Error: domainai.ErrVideoRefMixed, Code: "ai.video_ref_mixed", Kind: apperr.Validation, Message: domainai.ErrVideoRefMixed.Error()},
	{Error: domainai.ErrVideoRefPromptRequired, Code: "ai.video_ref_prompt_required", Kind: apperr.Validation, Message: domainai.ErrVideoRefPromptRequired.Error()},
}
