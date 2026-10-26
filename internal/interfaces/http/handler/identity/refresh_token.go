package identity

import (
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
		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	res.OkWithData(jwtToken, c)
}
