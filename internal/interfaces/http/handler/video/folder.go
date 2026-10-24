package video

import (
	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CreateFolderReq 提供新收藏夹名称，所有者来自认证上下文。
type CreateFolderReq struct {
	Name string `json:"name" binding:"required,min=1,max=50"`
}

// CreateFolderView 以当前用户身份创建收藏夹，不接收请求自报的所有者。
func (h *Handler) CreateFolderView(c *gin.Context) {
	req := middlewares.GetBound[CreateFolderReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	folder, err := h.folders.CreateFolder(c.Request.Context(), uid, req.Name)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(app.FolderDTO{
		ID:        folder.ID,
		UserID:    folder.UserID,
		Name:      folder.Name,
		IsDefault: folder.IsDefault,
	}, c)
}

// UpdateFolderReq 指定当前用户名下的收藏夹与新名称，用例继续检查归属。
type UpdateFolderReq struct {
	FolderID uint   `json:"folderId" binding:"required"`
	Name     string `json:"name" binding:"required,min=1,max=50"`
}

// UpdateFolderView 将当前用户、收藏夹 ID 与名称交给应用服务，返回更新结果。
func (h *Handler) UpdateFolderView(c *gin.Context) {
	req := middlewares.GetBound[UpdateFolderReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	folder, err := h.folders.UpdateFolder(c.Request.Context(), uid, req.FolderID, req.Name)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(app.FolderDTO{
		ID:        folder.ID,
		UserID:    folder.UserID,
		Name:      folder.Name,
		IsDefault: folder.IsDefault,
	}, c)
}

// DeleteFolderReq 指定待删除收藏夹，删除关联与统计调整由仓储/用例协调。
type DeleteFolderReq struct {
	FolderID uint `json:"folderId" binding:"required"`
}

// DeleteFolderView 调用收藏夹删除用例并返回结果，接口层不自行扣减视频收藏计数。
func (h *Handler) DeleteFolderView(c *gin.Context) {
	req := middlewares.GetBound[DeleteFolderReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.folders.DeleteFolder(c.Request.Context(), uid, req.FolderID); err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithMsg("删除成功", c)
}

// FolderListReq 可附带视频 ID，以便列表同时展示该视频在各收藏夹中的状态。
type FolderListReq struct {
	VideoID uint `form:"videoId" binding:"omitempty"`
}

// FolderListView 获取用户收藏夹列表及视频数量.
func (h *Handler) FolderListView(c *gin.Context) {
	req := middlewares.GetBound[FolderListReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	list, err := h.folders.ListFolders(c.Request.Context(), uid, req.VideoID)
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithList(list, int64(len(list)), c)
}

// FolderVideosReq 描述某收藏夹的作品分页和排序，列表可见性由应用用例判断。
type FolderVideosReq struct {
	FolderID uint `form:"folderId" binding:"required"`
	Page     int  `form:"page" binding:"omitempty,min=1"`
	PageSize int  `form:"pageSize" binding:"omitempty,min=1,max=50"`
	Sort     int  `form:"sort" binding:"omitempty,oneof=0 1 2"` // 0最近收藏 1最多播放 2最多点赞
}

// FolderVideosView 获取收藏夹内视频列表.
func (h *Handler) FolderVideosView(c *gin.Context) {
	req := middlewares.GetBound[FolderVideosReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	items, total, err := h.folders.ListFolderVideos(
		c.Request.Context(), uid, req.FolderID, req.Page, req.PageSize, req.Sort,
	)
	if err != nil {
		if isRecordNotFound(err) {
			res.FailWithMsg("收藏夹不存在", c)

			return
		}

		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithList(items, total, c)
}
