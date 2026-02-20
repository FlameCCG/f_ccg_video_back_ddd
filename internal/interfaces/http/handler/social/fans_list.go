package social

import (
	"strconv"
	"strings"

	app "f_ccg_video_back_ddd/internal/application/social"
	"f_ccg_video_back_ddd/pkg/res"
	"f_ccg_video_back_ddd/pkg/utils"

	"github.com/gin-gonic/gin"
)

// FansUserInfo 粉丝列表条目响应.
type FansUserInfo struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	Avatar      string `json:"avatar"`
	Description string `json:"description"`
	Level       int    `json:"level"`
}

// FansListView 获取粉丝列表（分页）.
// Query: id
func (h *Handler) FansListView(c *gin.Context) {
	userID := strings.TrimSpace(c.Query("id"))
	if userID == "" {
		res.FailWithMsg("用户ID不能为空", c)
		return
	}

	uid, err := strconv.Atoi(userID)
	if err != nil || uid <= 0 {
		res.FailWithMsg("用户ID格式错误", c)
		return
	}

	// 获取分页参数
	page, pageSize := utils.GetPageInfo(c)
	keyword := strings.TrimSpace(c.Query("keyword"))

	listRows, total, err := h.relations.FansList(c.Request.Context(), app.SocialListParams{
		OwnerID:  uid,
		ViewerID: viewerID(c),
		Page:     page,
		PageSize: pageSize,
		Keyword:  keyword,
	})
	if err != nil {
		res.FailWithMsg(err.Error(), c)
		return
	}

	// 组装返回数据
	list := make([]FansUserInfo, 0, len(listRows))
	for _, fans := range listRows {
		list = append(list, FansUserInfo{
			ID:          fans.ID,
			Username:    fans.Username,
			Avatar:      fans.Avatar,
			Description: fans.Description,
			Level:       fans.Level,
		})
	}

	res.OkWithList(list, total, c)
}
