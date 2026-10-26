package identity

import (
	"context"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

func serveAdminAccountList[P, T any](c *gin.Context, status enum.UserStatus, params P, query func(context.Context, P) ([]T, int64, error)) {
	if !ensureUserStatusValid(c, status) {
		return
	}

	rows, total, err := query(c.Request.Context(), params)
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)
		return
	}

	res.OkWithList(rows, total, c)
}
