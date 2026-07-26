package comment

import (
	app "f_ccg_video_back_ddd/internal/application/comment"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"
	"f_ccg_video_back_ddd/pkg/utils/ipUtils"

	"github.com/gin-gonic/gin"
)

// CreateCommentReq is an exported API type.
type CreateCommentReq struct {
	VideoID   uint     `json:"videoId" binding:"omitempty,gt=0" label:"视频ID"`
	DynamicID uint     `json:"dynamicId" binding:"omitempty,gt=0" label:"动态ID"`
	Content   string   `json:"content" binding:"max=1000" label:"评论内容"`
	ParentID  *uint    `json:"parentId" label:"父评论ID"`
	AtUserIDs []uint   `json:"atUserIds" label:"@的用户ID列表"`
	Pictures  []string `json:"pictures" label:"评论附带图片列表"`
}

// CreateView handles the corresponding HTTP request.
func (h *Handler) CreateView(c *gin.Context) {
	req := middlewares.GetBound[CreateCommentReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	ip, addr := ipUtils.GetClientIPAndAddr(c, h.resolver)

	if (req.VideoID > 0 && req.DynamicID > 0) || (req.VideoID == 0 && req.DynamicID == 0) {
		res.FailWithMsg("请传入 videoId 或 dynamicId", c)

		return
	}

	comment, err := h.svc.Create(c.Request.Context(), app.CreateCommand{
		UserID:    uid,
		VideoID:   req.VideoID,
		DynamicID: req.DynamicID,
		Content:   req.Content,
		ParentID:  req.ParentID,
		AtUserIDs: req.AtUserIDs,
		Pictures:  req.Pictures,
		IP:        ip,
		Addr:      addr,
	})
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithData(comment, c)
}
