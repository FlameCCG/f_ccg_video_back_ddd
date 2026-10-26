package rbac

import (
	"context"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/pkg/res"
	"strconv"

	"github.com/gin-gonic/gin"
)

func serveIDLookup[T any](c *gin.Context, key string, query func(context.Context, uint) (T, error)) {
	raw := c.Query(key)
	if raw == "" {
		res.FailWithMsg(key+" 不能为空", c)
		return
	}

	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		res.FailWithMsg(key+" 非法", c)
		return
	}

	result, err := query(c.Request.Context(), uint(id))
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)
		return
	}

	res.OkWithData(result, c)
}
