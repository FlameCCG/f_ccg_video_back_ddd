package comment

import (
	app "f_ccg_video_back_ddd/internal/application/comment"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CommentListReq 描述视频或动态的根评论分页与排序；评论目标合法性由应用/领域规则继续校验。
type CommentListReq struct {
	VideoID   uint   `form:"videoId" binding:"omitempty,gt=0" label:"视频ID"`
	DynamicID uint   `form:"dynamicId" binding:"omitempty,gt=0" label:"动态ID"`
	Page      int    `form:"page" label:"页码"`
	PageSize  int    `form:"pageSize" label:"每页数量"`
	SortBy    string `form:"sortBy" label:"排序方式"` // time/hot
}

// ListView 将列表参数交给评论查询用例，按统一分页信封返回。
func (h *Handler) ListView(c *gin.Context) {
	req := middlewares.GetBound[CommentListReq](c)
	if (req.VideoID > 0 && req.DynamicID > 0) || (req.VideoID == 0 && req.DynamicID == 0) {
		res.FailWithMsg("请传入 videoId 或 dynamicId", c)

		return
	}

	var uid uint
	if v, ok := middlewares.GetUID(c); ok {
		uid = v
	}

	list, total, err := h.svc.List(c.Request.Context(), app.ListParams{
		VideoID:   req.VideoID,
		DynamicID: req.DynamicID,
		ViewerID:  uid,
		Page:      req.Page,
		PageSize:  req.PageSize,
		SortBy:    req.SortBy,
	})
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithList(list, total, c)
}
