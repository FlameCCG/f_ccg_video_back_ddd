package social

import (
	"strconv"
	"strings"

	app "f_ccg_video_back_ddd/internal/application/social"
	"f_ccg_video_back_ddd/pkg/res"
	"f_ccg_video_back_ddd/pkg/utils"

	"github.com/gin-gonic/gin"
)

// FocusUserInfo 关注列表条目响应.
type FocusUserInfo struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	Avatar      string `json:"avatar"`
	Description string `json:"description"`
	Level       int    `json:"level"`
}

// FocusListView 获取关注列表（分页）.
// Query: id
func (h *Handler) FocusListView(c *gin.Context) {
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

	listRows, total, err := h.relations.FocusList(c.Request.Context(), app.SocialListParams{
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
	list := make([]FocusUserInfo, 0, len(listRows))
	for _, focus := range listRows {
		list = append(list, FocusUserInfo{
			ID:          focus.ID,
			Username:    focus.Username,
			Avatar:      focus.Avatar,
			Description: focus.Description,
			Level:       focus.Level,
		})
	}

	res.OkWithList(list, total, c)
}
