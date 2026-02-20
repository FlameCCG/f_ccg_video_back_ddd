package video

import (
	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// CreateFolderReq is an exported API type.
type CreateFolderReq struct {
	Name string `json:"name" binding:"required,min=1,max=50"`
}

// CreateFolderView handles the corresponding HTTP request.
func (h *Handler) CreateFolderView(c *gin.Context) {
	req := middlewares.GetBound[CreateFolderReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	folder, err := h.folders.CreateFolder(c.Request.Context(), uid, req.Name)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithData(app.FolderDTO{
		ID:        folder.ID,
		UserID:    folder.UserID,
		Name:      folder.Name,
		IsDefault: folder.IsDefault,
	}, c)
}

// DeleteFolderReq is an exported API type.
type DeleteFolderReq struct {
	FolderID uint `json:"folderId" binding:"required"`
}

// DeleteFolderView handles the corresponding HTTP request.
func (h *Handler) DeleteFolderView(c *gin.Context) {
	req := middlewares.GetBound[DeleteFolderReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.folders.DeleteFolder(c.Request.Context(), uid, req.FolderID); err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithMsg("删除成功", c)
}

// FolderListReq is an exported API type.
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
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithList(list, int64(len(list)), c)
}

// FolderVideosReq is an exported API type.
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

		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithList(items, total, c)
}
