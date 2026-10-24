package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// UserVideosReq 请求/响应结构体.
type UserVideosReq struct {
	UserID      uint `form:"userId" binding:"required,gt=0" label:"用户ID"`
	Page        int  `form:"page" binding:"omitempty,min=1"`
	Size        int  `form:"pageSize" binding:"omitempty,min=1,max=50"`
	Sort        int  `form:"sort" binding:"omitempty,oneof=0 1 2"`          // 0最新 1最多播放 2最多收藏
	AuditStatus int  `form:"auditStatus" binding:"omitempty,oneof=1 2 4 5"` // 1已发布 2私密 4审核中 5定时发布
}

// UserVideosView 用户主页视频列表.
func (h *Handler) UserVideosView(c *gin.Context) {
	req := middlewares.GetBound[UserVideosReq](c)
	viewerID, _ := middlewares.GetUID(c)

	list, total, err := h.activity.ListUserVideos(
		c.Request.Context(),
		req.UserID,
		viewerID,
		req.Page,
		req.Size,
		req.Sort,
		req.AuditStatus,
	)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithList(list, total, c)
}
