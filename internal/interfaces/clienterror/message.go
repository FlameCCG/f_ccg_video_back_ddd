// Package clienterror exposes only explicitly allowlisted business errors.
package clienterror

import (
	"errors"
	"log/slog"

	applicationcontentops "f_ccg_video_back_ddd/internal/application/contentops"
	applicationidentity "f_ccg_video_back_ddd/internal/application/identity"
	applicationnotification "f_ccg_video_back_ddd/internal/application/notification"
	applicationprovisioning "f_ccg_video_back_ddd/internal/application/provisioning"
	applicationsearch "f_ccg_video_back_ddd/internal/application/search"
	applicationvideo "f_ccg_video_back_ddd/internal/application/video"
	aierrors "f_ccg_video_back_ddd/internal/domain/ai/domainerr"
	chaterrors "f_ccg_video_back_ddd/internal/domain/chat/domainerr"
	commenterrors "f_ccg_video_back_ddd/internal/domain/comment/domainerr"
	contentopserrors "f_ccg_video_back_ddd/internal/domain/contentops/domainerr"
	dynamicerrors "f_ccg_video_back_ddd/internal/domain/dynamic/domainerr"
	identityerrors "f_ccg_video_back_ddd/internal/domain/identity/domainerr"
	identityentity "f_ccg_video_back_ddd/internal/domain/identity/entity"
	mediaerrors "f_ccg_video_back_ddd/internal/domain/media/domainerr"
	notificationentity "f_ccg_video_back_ddd/internal/domain/notification/entity"
	rbacerrors "f_ccg_video_back_ddd/internal/domain/rbac/domainerr"
	rbacvo "f_ccg_video_back_ddd/internal/domain/rbac/valueobject"
	socialerrors "f_ccg_video_back_ddd/internal/domain/social/domainerr"
	videoerrors "f_ccg_video_back_ddd/internal/domain/video/domainerr"
)

var publicErrors = []error{aierrors.ErrAssetHostNotAllowed, aierrors.ErrChatAPIKeyMissing, aierrors.ErrConfigMissing, aierrors.ErrDoubaoAPIKeyMissing, aierrors.ErrEmptyInput, aierrors.ErrEmptyPrompt, aierrors.ErrEmptyRequestID, aierrors.ErrImageCountRange, aierrors.ErrImageRefExclusive, aierrors.ErrImageRefPlusCount, aierrors.ErrNoUserMessage, aierrors.ErrOutputTextNotFound, aierrors.ErrTooManyImageRefs, aierrors.ErrTooManyVideoRefs, aierrors.ErrUnsupportedAssetScheme, aierrors.ErrVideoInputRequired, aierrors.ErrVideoRefMixed, aierrors.ErrVideoRefPromptRequired, chaterrors.ErrConversationNotFound, chaterrors.ErrDailySendLimit, chaterrors.ErrEmptyEmoji, chaterrors.ErrEmptyMediaURL, chaterrors.ErrEmptyPeerID, chaterrors.ErrEmptyText, chaterrors.ErrInvalidCursor, chaterrors.ErrInvalidPeerID, chaterrors.ErrNotLoggedIn, chaterrors.ErrSelfConversation, chaterrors.ErrSelfMessage, chaterrors.ErrStoreUnavailable, chaterrors.ErrTextTooLong, chaterrors.ErrUnsupportedMessageType, commenterrors.ErrAuthorIDRequired, commenterrors.ErrAuthorNotFound, commenterrors.ErrCommentIDRequired, commenterrors.ErrCommentIDsRequired, commenterrors.ErrCommentNotFound, commenterrors.ErrContentEmpty, commenterrors.ErrContentTooLong, commenterrors.ErrDeleteForbidden, commenterrors.ErrDynamicNotFound, commenterrors.ErrLikeCacheUnavailable, commenterrors.ErrLikeDuplicated, commenterrors.ErrParentNotFound, commenterrors.ErrPinForbidden, commenterrors.ErrPinRootOnly, commenterrors.ErrPinTargetMissing, commenterrors.ErrRecordNotFound, commenterrors.ErrTargetRequired, commenterrors.ErrUserNotLogin, commenterrors.ErrVideoNotPublished, contentopserrors.ErrBannerInvalidParam, contentopserrors.ErrBannerNoUpdatableField, contentopserrors.ErrBannerNotFound, contentopserrors.ErrBannerPartitionNotFound, contentopserrors.ErrBannerPartitionTypeUnsupported, dynamicerrors.ErrAuthorIDRequired, dynamicerrors.ErrContentEmpty, dynamicerrors.ErrContentTooLong, dynamicerrors.ErrDeleteForbidden, dynamicerrors.ErrDynamicIDRequired, dynamicerrors.ErrDynamicIDsRequired, dynamicerrors.ErrDynamicNotFound, dynamicerrors.ErrLikeDuplicated, dynamicerrors.ErrLoginRequired, dynamicerrors.ErrPinForbidden, dynamicerrors.ErrRecordNotFound, dynamicerrors.ErrUserIDRequired, dynamicerrors.ErrUserNotFound, dynamicerrors.ErrUserNotLogin, dynamicerrors.ErrVideoIDRequired, dynamicerrors.ErrVideoPinForbidden, identityerrors.ErrAccountBanned, identityerrors.ErrAccountPermanentlyBanned, identityentity.ErrBanDaysRequired, identityentity.ErrBanStatusInvalid, identityentity.ErrBanUserIDRequired, identityerrors.ErrBannerActionInvalid, identityerrors.ErrBannerAlreadyInDefaults, identityerrors.ErrBannerAlreadyRegisterDefault, identityerrors.ErrBannerIDRequired, identityerrors.ErrBannerIDsRequired, identityerrors.ErrBirthdayFormatInvalid, identityerrors.ErrCoinInsufficient, identityerrors.ErrCredentialsInvalid, identityerrors.ErrDuplicateRecord, identityerrors.ErrEmailNotRegistered, identityerrors.ErrEmailOccupied, identityerrors.ErrGenderInvalid, identityerrors.ErrIPRequired, identityerrors.ErrNoFieldsToUpdate, identityerrors.ErrOAuthOpenIDEmpty, identityerrors.ErrOAuthRegisterSourceEmpty, identityerrors.ErrOAuthStateInvalid, identityerrors.ErrOldPasswordIncorrect, identityerrors.ErrOldPasswordRequired, identityerrors.ErrPasswordHashFailed, identityerrors.ErrRecentCoinedNotPublic, identityerrors.ErrRecentLikedNotPublic, identityerrors.ErrRegisterDefaultBannerLocked, identityerrors.ErrRoleNotOwned, identityerrors.ErrSystemDefaultBannerNotFound, identityerrors.ErrUserBannerNotFound, identityerrors.ErrUserConfNotFound, identityerrors.ErrUserIDRequired, identityerrors.ErrUserNotFound, identityerrors.ErrUserNotLogin, identityerrors.ErrUserOrEmailExists, identityerrors.ErrUserRecordNotFound, identityerrors.ErrUsernameExists, mediaerrors.ErrTranscodeEncodeTimeout, mediaerrors.ErrTranscodeProgressStalled, mediaerrors.ErrTranscodeUploadTimeout, mediaerrors.ErrVideoDeleted, notificationentity.ErrSystemNotificationNotDeletable, rbacerrors.ErrAddFailed, rbacerrors.ErrAddPermissionFailed, rbacerrors.ErrAssignFailed, rbacerrors.ErrBindMenuFailed, rbacerrors.ErrBreakInheritanceFailed, rbacerrors.ErrCasbinNotInitialized, rbacerrors.ErrCreateFailed, rbacerrors.ErrDeleteChildMenusFirst, rbacerrors.ErrDeleteFailed, rbacerrors.ErrEnforcerNotInitialized, rbacerrors.ErrGetFailed, rbacerrors.ErrGetPermissionFailed, rbacerrors.ErrGetResourceFailed, rbacerrors.ErrGetWildcardFailed, rbacerrors.ErrInheritCycle, rbacerrors.ErrInheritFailed, rbacerrors.ErrInvalidCasbinPermission, rbacerrors.ErrMenuComponentEmpty, rbacerrors.ErrMenuCreatorRoleMissing, rbacerrors.ErrMenuExists, rbacerrors.ErrMenuGenerationTitleEmpty, rbacerrors.ErrMenuGeneratorUnavailable, rbacvo.ErrMenuIconTooLong, rbacvo.ErrMenuIconUnsafe, rbacerrors.ErrMenuMoveAcrossLevels, rbacerrors.ErrMenuMovePosition, rbacerrors.ErrMenuNameEmpty, rbacerrors.ErrMenuNotFound, rbacerrors.ErrMenuParentCycle, rbacerrors.ErrMenuPathEmpty, rbacerrors.ErrMenuTitleEmpty, rbacerrors.ErrMenuTitleEnEmpty, rbacerrors.ErrMenuTitleJaEmpty, rbacerrors.ErrNoRouteFound, rbacerrors.ErrNoUpdatableFields, rbacerrors.ErrNotDirectInherit, rbacerrors.ErrOpenAPINotFound, rbacerrors.ErrOpenAPIParse, rbacerrors.ErrOpenAPIRead, rbacerrors.ErrParentMenuIsSelf, rbacerrors.ErrParentMenuNotFound, rbacerrors.ErrParentRoleIsSelf, rbacerrors.ErrParentRoleNotFound, rbacerrors.ErrPartialMenusNotFound, rbacerrors.ErrPermissionAlreadyOwned, rbacerrors.ErrPermissionParamIncomplete, rbacerrors.ErrPermissionSaveFailed, rbacerrors.ErrPermissionSyncFailed, rbacerrors.ErrPermissionsRequired, rbacerrors.ErrPrepareMenuDataFailed, rbacerrors.ErrQueryFailed, rbacerrors.ErrQueryMenuFailed, rbacerrors.ErrQueryPermissionFailed, rbacerrors.ErrQueryRoleFailed, rbacerrors.ErrQueryUserFailed, rbacerrors.ErrReadMenuDataFailed, rbacerrors.ErrReadParentMenuFailed, rbacerrors.ErrRemoveFailed, rbacerrors.ErrRoleHasUsers, rbacerrors.ErrRoleInitFailed, rbacerrors.ErrRoleNameBlank, rbacerrors.ErrRoleNameEmpty, rbacerrors.ErrRoleNameExists, rbacerrors.ErrRoleNotFound, rbacerrors.ErrRolePolicyCleanupFailed, rbacerrors.ErrRolePolicySaveFailed, rbacerrors.ErrSaveFailed, rbacerrors.ErrSavePermissionFailed, rbacerrors.ErrSomeMenusNotFound, rbacerrors.ErrSomeRolesNotFound, rbacerrors.ErrSortFailed, rbacerrors.ErrTemplatePermissionsRead, rbacerrors.ErrTemplateRoleNotFound, rbacerrors.ErrUpdateFailed, rbacerrors.ErrUserNotFound, socialerrors.ErrAlreadyFocused, socialerrors.ErrCannotFocusSelf, socialerrors.ErrFansNotPublic, socialerrors.ErrFocusFailed, socialerrors.ErrFollowNotPublic, socialerrors.ErrForeignPageLimited, socialerrors.ErrListQueryFailed, socialerrors.ErrNotFocused, socialerrors.ErrSelfRelation, socialerrors.ErrTargetUserNotFound, socialerrors.ErrUnfocusFailed, socialerrors.ErrUserIDRequired, socialerrors.ErrUserNotFound, videoerrors.ErrActionUnsupported, videoerrors.ErrCoinCountInvalid, videoerrors.ErrCoinInsufficient, videoerrors.ErrCoinLimit, videoerrors.ErrCreatePartResourcesFailed, videoerrors.ErrCreatePartsFailed, videoerrors.ErrCreateVideoFailed, videoerrors.ErrCreateVideoResourceFailed, videoerrors.ErrCreateVideoTagsFailed, videoerrors.ErrDanmuContentEmpty, videoerrors.ErrDanmuDeleteForbidden, videoerrors.ErrDanmuIDRequired, videoerrors.ErrDanmuIDsRequired, videoerrors.ErrDanmuNotFound, videoerrors.ErrDanmuPartIDRequired, videoerrors.ErrDanmuPartNotFound, videoerrors.ErrDanmuPositionInvalid, videoerrors.ErrDanmuTimeOffsetNegative, videoerrors.ErrDefaultFolderUndeletable, videoerrors.ErrDuplicateCoin, videoerrors.ErrDuplicateLike, videoerrors.ErrFolderNameEmpty, videoerrors.ErrFolderNotFoundOrForbidden, videoerrors.ErrLoginRequired, videoerrors.ErrNotLoggedIn, videoerrors.ErrParamInvalid, videoerrors.ErrPartitionHasVideos, videoerrors.ErrPartitionInactive, videoerrors.ErrPartitionNotFound, videoerrors.ErrPartitionNotSubmittable, videoerrors.ErrRecordNotFound, videoerrors.ErrReportIDsRequired, videoerrors.ErrReportNoteTooLong, videoerrors.ErrReportReasonEmpty, videoerrors.ErrReportStatusInvalid, videoerrors.ErrReportTypeInvalid, videoerrors.ErrSchedulerNotRunning, videoerrors.ErrTagNameEmpty, videoerrors.ErrTagNameTooLong, videoerrors.ErrUserIDRequired, videoerrors.ErrUserNotFound, videoerrors.ErrUserNotLogin, videoerrors.ErrVideoAuthorNotFound, videoerrors.ErrVideoDeleteForbidden, videoerrors.ErrVideoIDRequired, videoerrors.ErrVideoIDsRequired, videoerrors.ErrVideoNotFound, videoerrors.ErrVideoNotPublished, videoerrors.ErrVideoStatusInvalid, applicationcontentops.ErrBannerPartitionFilterUnsupported,
	applicationcontentops.ErrConfigFileRead,
	applicationcontentops.ErrConfigFileWrite,
	applicationcontentops.ErrConfigItemUnsupported,
	applicationcontentops.ErrSiteConfigNotInitialized,
	applicationidentity.ErrAdminRoleMissing,
	applicationidentity.ErrBannerListFailed,
	applicationidentity.ErrBannerUploadFailed,
	applicationidentity.ErrBindEmailFailed,
	applicationidentity.ErrChangePasswordFailed,
	applicationidentity.ErrCreatorIDRequired,
	applicationidentity.ErrCreatorMetricTypeInvalid,
	applicationidentity.ErrCreatorRangeInvalid,
	applicationidentity.ErrEmailAlreadyBound,
	applicationidentity.ErrEmailAlreadyRegistered,
	applicationidentity.ErrEmailCaptchaEmailNotRegistered,
	applicationidentity.ErrEmailCaptchaRateLimited,
	applicationidentity.ErrEmailCaptchaStoreFailed,
	applicationidentity.ErrEmailHostNotExists,
	applicationidentity.ErrEmailNoMXRecords,
	applicationidentity.ErrEmailSyntaxInvalid,
	applicationidentity.ErrEmailVerifyFailed,
	applicationidentity.ErrLoginFailed,
	applicationidentity.ErrNotAdminAccount,
	applicationidentity.ErrOAuthAccountBindingRequired,
	applicationidentity.ErrOAuthProviderFailed,
	applicationidentity.ErrOAuthProviderUnsupported,
	applicationidentity.ErrPersistSiteConfigFailed,
	applicationidentity.ErrPwdLoginDisabled,
	applicationidentity.ErrQueryUserFailed,
	applicationidentity.ErrRefreshTokenFailed,
	applicationidentity.ErrRefreshTokenInvalid,
	applicationidentity.ErrRegisterFailed,
	applicationidentity.ErrResetPasswordFailed,
	applicationidentity.ErrRoleCheckFailed,
	applicationidentity.ErrSetUserBannerFailed,
	applicationidentity.ErrSiteConfigNotInitialized,
	applicationidentity.ErrStorageConfigInvalid,
	applicationidentity.ErrSwitchRoleFailed,
	applicationidentity.ErrUpdateUserFailed,
	applicationnotification.ErrNoticeNotFound,
	applicationnotification.ErrNoticeQueryFailed,
	applicationnotification.ErrNoticeUpdateFailed,
	applicationprovisioning.ErrBannerImagesDirUnreadable,
	applicationprovisioning.ErrSuperUserArgsRequired,
	applicationsearch.ErrAIConfigMissing,
	applicationsearch.ErrES8Unavailable,
	applicationsearch.ErrESUnavailable,
	applicationsearch.ErrEmbeddingAPIKeyMissing,
	applicationsearch.ErrEmbeddingModelMissing,
	applicationsearch.ErrInvalidMentionUserID,
	applicationsearch.ErrUnexpectedMultiSearchSize,
	applicationsearch.ErrVectorSourceNotFound,
	applicationvideo.ErrDailyUploadLimit,
	applicationvideo.ErrUploadQuotaUnavailable,
	applicationvideo.ErrUploadStorageUnavailable,
	applicationvideo.ErrVectorRebuildInProgress,
}

// Message never returns the outer error, which can contain SQL, storage keys or addresses.
func Message(err error) string {
	for _, known := range publicErrors {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	if err != nil {
		slog.Error("request failed", "error", err)
	}
	return "操作失败"
}
