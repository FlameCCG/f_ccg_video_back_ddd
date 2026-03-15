package social

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"strconv"
	"strings"

	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// FocusStatusRes 关注关系响应.
type FocusStatusRes struct {
	IsFocus        bool `json:"isFocus"`        // 我是否关注了他
	IsFans         bool `json:"isFans"`         // 他是否关注了我
	IsMutualFollow bool `json:"isMutualFollow"` // 是否互相关注
}

// FocusStatusView 查询与指定用户的关注关系.
// Query: id
func (h *Handler) FocusStatusView(c *gin.Context) {
	targetUserID := strings.TrimSpace(c.Query("id"))
	if targetUserID == "" {
		res.FailWithMsg("用户ID不能为空", c)
		return
	}

	targetUID, err := strconv.Atoi(targetUserID)
	if err != nil {
		res.FailWithMsg("用户ID格式错误", c)
		return
	}

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	relation, err := h.relations.FocusStatus(c.Request.Context(), uid, targetUID)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)
		return
	}

	result := FocusStatusRes{
		IsFocus:        relation.IsFocus,
		IsFans:         relation.IsFans,
		IsMutualFollow: relation.IsMutual(),
	}

	res.OkWithData(result, c)
}
