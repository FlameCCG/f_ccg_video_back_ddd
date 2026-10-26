package clienterror

import (
	domainidentity "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/pkg/apperr"
)

// domainidentityDescriptions explicitly classifies safe messages in this business context.
var domainidentityDescriptions = []Descriptor{
	{Error: domainidentity.ErrAccountBanned, Code: "identity.account_banned", Kind: apperr.Rejected, Message: domainidentity.ErrAccountBanned.Error()},
	{Error: domainidentity.ErrAccountPermanentlyBanned, Code: "identity.account_permanently_banned", Kind: apperr.Rejected, Message: domainidentity.ErrAccountPermanentlyBanned.Error()},
	{Error: domainidentity.ErrBanDaysRequired, Code: "identity.ban_days_required", Kind: apperr.Validation, Message: domainidentity.ErrBanDaysRequired.Error()},
	{Error: domainidentity.ErrBanStatusInvalid, Code: "identity.ban_status_invalid", Kind: apperr.Validation, Message: domainidentity.ErrBanStatusInvalid.Error()},
	{Error: domainidentity.ErrBanUserIDRequired, Code: "identity.ban_user_idrequired", Kind: apperr.Validation, Message: domainidentity.ErrBanUserIDRequired.Error()},
	{Error: domainidentity.ErrBannerActionInvalid, Code: "identity.banner_action_invalid", Kind: apperr.Validation, Message: domainidentity.ErrBannerActionInvalid.Error()},
	{Error: domainidentity.ErrBannerAlreadyInDefaults, Code: "identity.banner_already_in_defaults", Kind: apperr.Conflict, Message: domainidentity.ErrBannerAlreadyInDefaults.Error()},
	{Error: domainidentity.ErrBannerAlreadyRegisterDefault, Code: "identity.banner_already_register_default", Kind: apperr.Conflict, Message: domainidentity.ErrBannerAlreadyRegisterDefault.Error()},
	{Error: domainidentity.ErrBannerIDRequired, Code: "identity.banner_idrequired", Kind: apperr.Validation, Message: domainidentity.ErrBannerIDRequired.Error()},
	{Error: domainidentity.ErrBannerIDsRequired, Code: "identity.banner_ids_required", Kind: apperr.Validation, Message: domainidentity.ErrBannerIDsRequired.Error()},
	{Error: domainidentity.ErrBirthdayFormatInvalid, Code: "identity.birthday_format_invalid", Kind: apperr.Validation, Message: domainidentity.ErrBirthdayFormatInvalid.Error()},
	{Error: domainidentity.ErrCoinInsufficient, Code: "identity.coin_insufficient", Kind: apperr.Rejected, Message: domainidentity.ErrCoinInsufficient.Error()},
	{Error: domainidentity.ErrCredentialsInvalid, Code: "identity.credentials_invalid", Kind: apperr.Validation, Message: domainidentity.ErrCredentialsInvalid.Error()},
	{Error: domainidentity.ErrDuplicateRecord, Code: "identity.duplicate_record", Kind: apperr.Conflict, Message: domainidentity.ErrDuplicateRecord.Error()},
	{Error: domainidentity.ErrEmailNotRegistered, Code: "identity.email_not_registered", Kind: apperr.Rejected, Message: domainidentity.ErrEmailNotRegistered.Error()},
	{Error: domainidentity.ErrEmailOccupied, Code: "identity.email_occupied", Kind: apperr.Conflict, Message: domainidentity.ErrEmailOccupied.Error()},
	{Error: domainidentity.ErrGenderInvalid, Code: "identity.gender_invalid", Kind: apperr.Validation, Message: domainidentity.ErrGenderInvalid.Error()},
	{Error: domainidentity.ErrIPRequired, Code: "identity.iprequired", Kind: apperr.Validation, Message: domainidentity.ErrIPRequired.Error()},
	{Error: domainidentity.ErrNoFieldsToUpdate, Code: "identity.no_fields_to_update", Kind: apperr.Rejected, Message: domainidentity.ErrNoFieldsToUpdate.Error()},
	{Error: domainidentity.ErrOAuthOpenIDEmpty, Code: "identity.oauth_open_idempty", Kind: apperr.Validation, Message: domainidentity.ErrOAuthOpenIDEmpty.Error()},
	{Error: domainidentity.ErrOAuthRegisterSourceEmpty, Code: "identity.oauth_register_source_empty", Kind: apperr.Validation, Message: domainidentity.ErrOAuthRegisterSourceEmpty.Error()},
	{Error: domainidentity.ErrOAuthStateInvalid, Code: "identity.oauth_state_invalid", Kind: apperr.Validation, Message: domainidentity.ErrOAuthStateInvalid.Error()},
	{Error: domainidentity.ErrOldPasswordIncorrect, Code: "identity.old_password_incorrect", Kind: apperr.Rejected, Message: domainidentity.ErrOldPasswordIncorrect.Error()},
	{Error: domainidentity.ErrOldPasswordRequired, Code: "identity.old_password_required", Kind: apperr.Validation, Message: domainidentity.ErrOldPasswordRequired.Error()},
	{Error: domainidentity.ErrPasswordHashFailed, Code: "identity.password_hash_failed", Kind: apperr.Internal, Message: domainidentity.ErrPasswordHashFailed.Error()},
	{Error: domainidentity.ErrRecentCoinedNotPublic, Code: "identity.recent_coined_not_public", Kind: apperr.Rejected, Message: domainidentity.ErrRecentCoinedNotPublic.Error()},
	{Error: domainidentity.ErrRecentLikedNotPublic, Code: "identity.recent_liked_not_public", Kind: apperr.Rejected, Message: domainidentity.ErrRecentLikedNotPublic.Error()},
	{Error: domainidentity.ErrRegisterDefaultBannerLocked, Code: "identity.register_default_banner_locked", Kind: apperr.Rejected, Message: domainidentity.ErrRegisterDefaultBannerLocked.Error()},
	{Error: domainidentity.ErrRoleNotOwned, Code: "identity.role_not_owned", Kind: apperr.Rejected, Message: domainidentity.ErrRoleNotOwned.Error()},
	{Error: domainidentity.ErrSystemDefaultBannerNotFound, Code: "identity.system_default_banner_not_found", Kind: apperr.NotFound, Message: domainidentity.ErrSystemDefaultBannerNotFound.Error()},
	{Error: domainidentity.ErrUserBannerNotFound, Code: "identity.user_banner_not_found", Kind: apperr.NotFound, Message: domainidentity.ErrUserBannerNotFound.Error()},
	{Error: domainidentity.ErrUserConfNotFound, Code: "identity.user_conf_not_found", Kind: apperr.NotFound, Message: domainidentity.ErrUserConfNotFound.Error()},
	{Error: domainidentity.ErrUserIDRequired, Code: "identity.user_idrequired", Kind: apperr.Validation, Message: domainidentity.ErrUserIDRequired.Error()},
	{Error: domainidentity.ErrUserNotFound, Code: "identity.user_not_found", Kind: apperr.NotFound, Message: domainidentity.ErrUserNotFound.Error()},
	{Error: domainidentity.ErrUserNotLogin, Code: "identity.user_not_login", Kind: apperr.Rejected, Message: domainidentity.ErrUserNotLogin.Error()},
	{Error: domainidentity.ErrUserOrEmailExists, Code: "identity.user_or_email_exists", Kind: apperr.Conflict, Message: domainidentity.ErrUserOrEmailExists.Error()},
	{Error: domainidentity.ErrUserRecordNotFound, Code: "identity.user_record_not_found", Kind: apperr.NotFound, Message: domainidentity.ErrUserRecordNotFound.Error()},
	{Error: domainidentity.ErrUsernameExists, Code: "identity.username_exists", Kind: apperr.Conflict, Message: domainidentity.ErrUsernameExists.Error()},
}
