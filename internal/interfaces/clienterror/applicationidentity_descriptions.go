package clienterror

import (
	applicationidentity "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// applicationidentityDescriptions explicitly classifies safe messages in this business context.
var applicationidentityDescriptions = []Descriptor{
	{Error: applicationidentity.ErrAdminRoleMissing, Code: "identity.app_admin_role_missing", Kind: apperr.Rejected, Message: applicationidentity.ErrAdminRoleMissing.Error()},
	{Error: applicationidentity.ErrBannerListFailed, Code: "identity.app_banner_list_failed", Kind: apperr.Internal, Message: applicationidentity.ErrBannerListFailed.Error()},
	{Error: applicationidentity.ErrBannerUploadFailed, Code: "identity.app_banner_upload_failed", Kind: apperr.Internal, Message: applicationidentity.ErrBannerUploadFailed.Error()},
	{Error: applicationidentity.ErrBindEmailFailed, Code: "identity.app_bind_email_failed", Kind: apperr.Internal, Message: applicationidentity.ErrBindEmailFailed.Error()},
	{Error: applicationidentity.ErrChangePasswordFailed, Code: "identity.app_change_password_failed", Kind: apperr.Internal, Message: applicationidentity.ErrChangePasswordFailed.Error()},
	{Error: applicationidentity.ErrCreatorIDRequired, Code: "identity.app_creator_idrequired", Kind: apperr.Validation, Message: applicationidentity.ErrCreatorIDRequired.Error()},
	{Error: applicationidentity.ErrCreatorMetricTypeInvalid, Code: "identity.app_creator_metric_type_invalid", Kind: apperr.Validation, Message: applicationidentity.ErrCreatorMetricTypeInvalid.Error()},
	{Error: applicationidentity.ErrCreatorRangeInvalid, Code: "identity.app_creator_range_invalid", Kind: apperr.Validation, Message: applicationidentity.ErrCreatorRangeInvalid.Error()},
	{Error: applicationidentity.ErrEmailAlreadyBound, Code: "identity.app_email_already_bound", Kind: apperr.Conflict, Message: applicationidentity.ErrEmailAlreadyBound.Error()},
	{Error: applicationidentity.ErrEmailAlreadyRegistered, Code: "identity.app_email_already_registered", Kind: apperr.Conflict, Message: applicationidentity.ErrEmailAlreadyRegistered.Error()},
	{Error: applicationidentity.ErrEmailCaptchaEmailNotRegistered, Code: "identity.app_email_captcha_email_not_registered", Kind: apperr.Rejected, Message: applicationidentity.ErrEmailCaptchaEmailNotRegistered.Error()},
	{Error: applicationidentity.ErrEmailCaptchaRateLimited, Code: "identity.app_email_captcha_rate_limited", Kind: apperr.RateLimited, Message: applicationidentity.ErrEmailCaptchaRateLimited.Error()},
	{Error: applicationidentity.ErrEmailCaptchaStoreFailed, Code: "identity.app_email_captcha_store_failed", Kind: apperr.Internal, Message: applicationidentity.ErrEmailCaptchaStoreFailed.Error()},
	{Error: applicationidentity.ErrEmailHostNotExists, Code: "identity.app_email_host_not_exists", Kind: apperr.Conflict, Message: applicationidentity.ErrEmailHostNotExists.Error()},
	{Error: applicationidentity.ErrEmailNoMXRecords, Code: "identity.app_email_no_mxrecords", Kind: apperr.Rejected, Message: applicationidentity.ErrEmailNoMXRecords.Error()},
	{Error: applicationidentity.ErrEmailSyntaxInvalid, Code: "identity.app_email_syntax_invalid", Kind: apperr.Validation, Message: applicationidentity.ErrEmailSyntaxInvalid.Error()},
	{Error: applicationidentity.ErrEmailVerifyFailed, Code: "identity.app_email_verify_failed", Kind: apperr.Internal, Message: applicationidentity.ErrEmailVerifyFailed.Error()},
	{Error: applicationidentity.ErrLoginFailed, Code: "identity.app_login_failed", Kind: apperr.Internal, Message: applicationidentity.ErrLoginFailed.Error()},
	{Error: applicationidentity.ErrNotAdminAccount, Code: "identity.app_not_admin_account", Kind: apperr.Rejected, Message: applicationidentity.ErrNotAdminAccount.Error()},
	{Error: applicationidentity.ErrOAuthAccountBindingRequired, Code: "identity.app_oauth_account_binding_required", Kind: apperr.Validation, Message: applicationidentity.ErrOAuthAccountBindingRequired.Error()},
	{Error: applicationidentity.ErrOAuthProviderFailed, Code: "identity.app_oauth_provider_failed", Kind: apperr.Internal, Message: applicationidentity.ErrOAuthProviderFailed.Error()},
	{Error: applicationidentity.ErrOAuthProviderUnsupported, Code: "identity.app_oauth_provider_unsupported", Kind: apperr.Validation, Message: applicationidentity.ErrOAuthProviderUnsupported.Error()},
	{Error: applicationidentity.ErrPersistSiteConfigFailed, Code: "identity.app_persist_site_config_failed", Kind: apperr.Internal, Message: applicationidentity.ErrPersistSiteConfigFailed.Error()},
	{Error: applicationidentity.ErrPwdLoginDisabled, Code: "identity.app_pwd_login_disabled", Kind: apperr.Rejected, Message: applicationidentity.ErrPwdLoginDisabled.Error()},
	{Error: applicationidentity.ErrQueryUserFailed, Code: "identity.app_query_user_failed", Kind: apperr.Internal, Message: applicationidentity.ErrQueryUserFailed.Error()},
	{Error: applicationidentity.ErrRefreshTokenFailed, Code: "identity.app_refresh_token_failed", Kind: apperr.Internal, Message: applicationidentity.ErrRefreshTokenFailed.Error()},
	{Error: applicationidentity.ErrRefreshTokenInvalid, Code: "identity.app_refresh_token_invalid", Kind: apperr.Validation, Message: applicationidentity.ErrRefreshTokenInvalid.Error()},
	{Error: applicationidentity.ErrRegisterFailed, Code: "identity.app_register_failed", Kind: apperr.Internal, Message: applicationidentity.ErrRegisterFailed.Error()},
	{Error: applicationidentity.ErrResetPasswordFailed, Code: "identity.app_reset_password_failed", Kind: apperr.Internal, Message: applicationidentity.ErrResetPasswordFailed.Error()},
	{Error: applicationidentity.ErrRoleCheckFailed, Code: "identity.app_role_check_failed", Kind: apperr.Internal, Message: applicationidentity.ErrRoleCheckFailed.Error()},
	{Error: applicationidentity.ErrSetUserBannerFailed, Code: "identity.app_set_user_banner_failed", Kind: apperr.Internal, Message: applicationidentity.ErrSetUserBannerFailed.Error()},
	{Error: applicationidentity.ErrSiteConfigNotInitialized, Code: "identity.app_site_config_not_initialized", Kind: apperr.Internal, Message: applicationidentity.ErrSiteConfigNotInitialized.Error()},
	{Error: applicationidentity.ErrStorageConfigInvalid, Code: "identity.app_storage_config_invalid", Kind: apperr.Validation, Message: applicationidentity.ErrStorageConfigInvalid.Error()},
	{Error: applicationidentity.ErrSwitchRoleFailed, Code: "identity.app_switch_role_failed", Kind: apperr.Internal, Message: applicationidentity.ErrSwitchRoleFailed.Error()},
	{Error: applicationidentity.ErrUpdateUserFailed, Code: "identity.app_update_user_failed", Kind: apperr.Internal, Message: applicationidentity.ErrUpdateUserFailed.Error()},
}
