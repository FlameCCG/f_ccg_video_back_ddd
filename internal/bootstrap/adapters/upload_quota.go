package adapters

import (
	"context"

	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	videoapp "f_ccg_video_back_ddd/internal/application/video"
)

// UploadQuotaExemption 将身份上下文的管理员判定翻译成视频上下文的额度豁免。
type UploadQuotaExemption struct {
	roles identityapp.UserRoleQuery
}

var _ videoapp.UploadQuotaExemption = (*UploadQuotaExemption)(nil)

func NewUploadQuotaExemption(roles identityapp.UserRoleQuery) *UploadQuotaExemption {
	return &UploadQuotaExemption{roles: roles}
}

func (a *UploadQuotaExemption) IsUploadQuotaExempt(ctx context.Context, userID uint) (bool, error) {
	return identityapp.IsAdminUser(ctx, a.roles, userID)
}
