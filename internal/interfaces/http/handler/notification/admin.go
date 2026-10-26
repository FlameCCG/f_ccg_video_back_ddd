package notification

import (
	"errors"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	app "f_ccg_video_back_ddd/internal/application/notification"

	"github.com/gin-gonic/gin"
)

// AdminListReq 指定管理端系统通知列表的分页窗口。
type AdminListReq struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"pageSize" binding:"omitempty,min=1,max=100"`
}

// AdminList 管理员查询所有系统消息.
func (h *Handler) AdminList(c *gin.Context) {
	req := middlewares.GetBound[AdminListReq](c)

	list, total, err := h.svc.AdminList(c.Request.Context(), req.Page, req.PageSize)
	if err != nil {
		res.FailWithCause(err, "获取失败", c)

		return
	}

	res.OkWithList(list, total, c)
}

// CreateNotificationReq 表达系统通知内容与目标接收者；ReceiverID=-1 表示全员通知，操作者身份另从上下文取得。
type CreateNotificationReq struct {
	Title      string `json:"title" binding:"required,max=200" label:"标题"`
	Content    string `json:"content" binding:"required" label:"内容"`
	ReceiverID int    `json:"receiverId" binding:"required" label:"接收者ID(-1为全员)"`
	VideoID    uint   `json:"videoId" binding:"omitempty"`
	VideoTitle string `json:"videoTitle" binding:"omitempty,max=200"`
	Link       string `json:"link" binding:"omitempty,max=500"`
}

// Create 系统通知（仅管理员）.
func (h *Handler) Create(c *gin.Context) {
	req := middlewares.GetBound[CreateNotificationReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.svc.CreateSystemNotice(c.Request.Context(), app.CreateSystemNoticeCommand{
		ActorID:    uid,
		Title:      req.Title,
		Content:    req.Content,
		ReceiverID: req.ReceiverID,
		VideoID:    req.VideoID,
		VideoTitle: req.VideoTitle,
		Link:       req.Link,
	}); err != nil {
		res.FailWithCause(err, "创建失败", c)

		return
	}

	res.OkWithMsg("创建成功", c)
}

// AdminUpdateReq 携带待更新系统通知 ID 与可编辑内容；记录存在性和通知类型由应用用例检查。
type AdminUpdateReq struct {
	ID         uint   `json:"id" binding:"required,gt=0"`
	Title      string `json:"title" binding:"required,max=200"`
	Content    string `json:"content" binding:"required"`
	ReceiverID int    `json:"receiverId" binding:"required" label:"接收者ID(-1为全员)"`
	VideoID    uint   `json:"videoId" binding:"omitempty"`
	VideoTitle string `json:"videoTitle" binding:"omitempty,max=200"`
	Link       string `json:"link" binding:"omitempty,max=500"`
}

// AdminUpdate 仅管理员修改系统通知.
func (h *Handler) AdminUpdate(c *gin.Context) {
	req := middlewares.GetBound[AdminUpdateReq](c)

	err := h.svc.UpdateSystemNotice(c.Request.Context(), app.UpdateSystemNoticeCommand{
		ID:         req.ID,
		Title:      req.Title,
		Content:    req.Content,
		ReceiverID: req.ReceiverID,
		VideoID:    req.VideoID,
		VideoTitle: req.VideoTitle,
		Link:       req.Link,
	})
	if err != nil {
		// 与原实现一致：查询失败 / 不存在 / 更新失败三种文案分别返回。
		switch {
		case errors.Is(err, app.ErrNoticeQueryFailed):
			res.FailWithCause(err, "查询失败", c)
		case errors.Is(err, app.ErrNoticeNotFound):
			res.FailWithCause(err, "通知不存在", c)
		default:
			res.FailWithCause(err, "更新失败", c)
		}

		return
	}

	res.OkWithMsg("更新成功", c)
}
