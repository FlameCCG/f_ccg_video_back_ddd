package clienterror

import (
	applicationsearch "f_ccg_video_back_ddd/internal/application/search"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// applicationsearchDescriptions explicitly classifies safe messages in this business context.
var applicationsearchDescriptions = []Descriptor{
	{Error: applicationsearch.ErrAIConfigMissing, Code: "search.app_aiconfig_missing", Kind: apperr.Internal, Message: applicationsearch.ErrAIConfigMissing.Error()},
	{Error: applicationsearch.ErrES8Unavailable, Code: "search.app_es8_unavailable", Kind: apperr.Dependency, Message: applicationsearch.ErrES8Unavailable.Error()},
	{Error: applicationsearch.ErrESUnavailable, Code: "search.app_esunavailable", Kind: apperr.Dependency, Message: applicationsearch.ErrESUnavailable.Error()},
	{Error: applicationsearch.ErrEmbeddingAPIKeyMissing, Code: "search.app_embedding_apikey_missing", Kind: apperr.Internal, Message: applicationsearch.ErrEmbeddingAPIKeyMissing.Error()},
	{Error: applicationsearch.ErrEmbeddingModelMissing, Code: "search.app_embedding_model_missing", Kind: apperr.Internal, Message: applicationsearch.ErrEmbeddingModelMissing.Error()},
	{Error: applicationsearch.ErrInvalidMentionUserID, Code: "search.app_invalid_mention_user_id", Kind: apperr.Validation, Message: applicationsearch.ErrInvalidMentionUserID.Error()},
	{Error: applicationsearch.ErrUnexpectedMultiSearchSize, Code: "search.app_unexpected_multi_search_size", Kind: apperr.Internal, Message: applicationsearch.ErrUnexpectedMultiSearchSize.Error()},
	{Error: applicationsearch.ErrVectorSourceNotFound, Code: "search.app_vector_source_not_found", Kind: apperr.NotFound, Message: applicationsearch.ErrVectorSourceNotFound.Error()},
}
