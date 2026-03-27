package identity

import (
	"errors"

	identityapp "f_ccg_video_back_ddd/internal/application/identity"
	identitydomain "f_ccg_video_back_ddd/internal/domain/identity"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// RefreshTokenReq 请求/响应结构体.
type RefreshTokenReq struct {
	RefreshToken string `json:"refreshToken" binding:"required" label:"刷新令牌"`
}

// RefreshTokenView 刷新访问令牌.
func (h *Handler) RefreshTokenView(c *gin.Context) {
	req := middlewares.GetBound[RefreshTokenReq](c)

	jwtToken, err := h.logins.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, identityapp.ErrRefreshTokenInvalid), errors.Is(err, identityapp.ErrRefreshTokenReused):
			res.FailWithMsg(identityapp.ErrRefreshTokenInvalid.Error(), c)
		case errors.Is(err, identitydomain.ErrUserNotFound), errors.Is(err, identitydomain.ErrAccountBanned), errors.Is(err, identitydomain.ErrAccountPermanentlyBanned):
			res.FailWithMsg(clienterror.Message(err), c)
		default:
			writeAuthDependencyError(c, err)
		}

		return
	}

	res.OkWithData(jwtToken, c)
}
