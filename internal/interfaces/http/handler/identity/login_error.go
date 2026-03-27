package identity

import (
	"errors"
	"net/http"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

func writeLoginError(c *gin.Context, err error) {
	if errors.Is(err, app.ErrSessionIssueFailed) {
		writeAuthDependencyError(c, err)
		return
	}
	res.FailWithMsg(clienterror.Message(err), c)
}

func writeAuthDependencyError(c *gin.Context, err error) {
	status, message := http.StatusServiceUnavailable, "身份验证暂时不可用，请稍后重试"
	if ctxutil.IsTimeout(err) {
		status, message = http.StatusGatewayTimeout, "身份验证超时，请稍后重试"
	}
	c.JSON(status, res.Response{Code: res.FailServiceCode, Data: map[string]any{}, Msg: message})
}
