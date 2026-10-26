package video

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SaveHistoryReq 记录作品播放进度与总时长，当前用户从认证上下文取得，单位为秒。
type SaveHistoryReq struct {
	VideoID  uint `json:"videoId" binding:"required" label:"视频ID"`
	Progress int  `json:"progress" binding:"min=0" label:"播放进度"` // 允许 0
	Duration int  `json:"duration" binding:"required,min=0" label:"视频总时长"`
}

// SaveHistoryView 保存/更新播放进度.
func (h *Handler) SaveHistoryView(c *gin.Context) {
	req := middlewares.GetBound[SaveHistoryReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.history.SavePlayHistory(c.Request.Context(), uid, req.VideoID, req.Progress, req.Duration); err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithMsg("保存成功", c)
}

// HistoryListReq 描述当前用户历史记录的分页与标题关键字筛选。
type HistoryListReq struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"pageSize" binding:"omitempty,min=1,max=50"`
	Keyword  string `form:"keyword" binding:"omitempty,max=100"`
}

// DeleteHistoryReq 指定当前用户要清理历史的作品 ID 集合，不表示删除作品本身。
type DeleteHistoryReq struct {
	VideoIDs []uint `json:"videoIds" binding:"required,min=1,dive,gt=0"`
}

// GetVideoHistoryView 获取用户历史记录列表.
func (h *Handler) GetVideoHistoryView(c *gin.Context) {
	req := middlewares.GetBound[HistoryListReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	list, total, err := h.history.ListUserHistory(c.Request.Context(), uid, req.Page, req.PageSize, req.Keyword)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithList(list, total, c)
}

// DeleteHistoryView 批量删除历史记录.
func (h *Handler) DeleteHistoryView(c *gin.Context) {
	req := middlewares.GetBound[DeleteHistoryReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.history.DeleteHistory(c.Request.Context(), uid, req.VideoIDs); err != nil {
		h.log().Error("delete history failed", zap.Error(err), zap.Uint("userId", uid))
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithMsg("删除成功", c)
}
