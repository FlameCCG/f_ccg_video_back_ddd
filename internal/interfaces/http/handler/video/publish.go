package video

import (
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PublishVideoReq is an exported API type.
type PublishVideoReq struct {
	Title       string                `json:"title" binding:"required,max=200" label:"视频标题"`
	Description string                `json:"description" binding:"max=5000" label:"视频描述"`
	PartitionID uint                  `json:"partitionId" binding:"required" label:"分区ID"`
	TagIDs      []uint                `json:"tags" binding:"max=10" label:"标签列表"`
	IsOriginal  bool                  `json:"isOriginal" label:"是否原创"`
	IsPrivate   bool                  `json:"isPrivate" label:"是否私密"`
	CoverURL    string                `json:"coverUrl" binding:"required" label:"封面图片URL"`
	FilePath    string                `json:"filePath" label:"文件路径"`
	FileName    string                `json:"fileName" binding:"omitempty,max=255" label:"源文件名"`
	FileHash    string                `json:"fileHash" label:"文件hash"`
	Parts       []VideoPartPublishReq `json:"parts" label:"分P列表"`
	PublishTime time.Time             `json:"publishTime" label:"定时发布时间"`
}

// VideoPartPublishReq is an exported API type.
type VideoPartPublishReq struct {
	Title    string `json:"title" binding:"max=200" label:"分P标题"`
	FilePath string `json:"filePath" label:"文件路径"`
	FileName string `json:"fileName" binding:"required,max=255" label:"源文件名"`
	FileHash string `json:"fileHash" binding:"required" label:"文件hash"`
}

// PublishVideoResp 视频投稿响应.
type PublishVideoResp struct {
	VideoID     uint       `json:"videoId"`
	Status      int        `json:"status"`
	PublishTime *time.Time `json:"publishTime,omitempty"`
	Cover       string     `json:"cover"`
}

// PublishVideoView handles the corresponding HTTP request.
func (h *Handler) PublishVideoView(c *gin.Context) {
	req := middlewares.GetBound[PublishVideoReq](c)

	username, ok := c.Get("username")
	if !ok {
		res.FailWithMsg("用户未登录", c)

		return
	}

	authorUsername, _ := username.(string)

	parts := make([]app.PublishPartInput, 0, len(req.Parts))
	for _, p := range req.Parts {
		parts = append(parts, app.PublishPartInput{
			Title:    p.Title,
			FilePath: p.FilePath,
			FileName: p.FileName,
			FileHash: p.FileHash,
		})
	}

	result, err := h.publish.PublishVideo(c.Request.Context(), app.PublishVideoCommand{
		Title:          req.Title,
		Description:    req.Description,
		PartitionID:    req.PartitionID,
		TagIDs:         req.TagIDs,
		IsOriginal:     req.IsOriginal,
		IsPrivate:      req.IsPrivate,
		CoverURL:       req.CoverURL,
		FilePath:       req.FilePath,
		FileName:       req.FileName,
		FileHash:       req.FileHash,
		Parts:          parts,
		PublishTime:    req.PublishTime,
		AuthorUsername: authorUsername,
	})
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	publishTime := result.PublishTime

	res.OkWithData(PublishVideoResp{
		VideoID:     result.VideoID,
		Status:      int(result.Status),
		PublishTime: &publishTime,
		Cover:       result.Cover,
	}, c)
}

// UpdateVideoReq is an exported API type.
type UpdateVideoReq struct {
	VideoID     uint       `json:"videoId" binding:"required"`
	Title       *string    `json:"title" binding:"omitempty,max=200"`
	Description *string    `json:"description" binding:"omitempty,max=5000"`
	PartitionID *uint      `json:"partitionId"`
	TagIDs      []uint     `json:"tags" binding:"max=10"`
	IsOriginal  *bool      `json:"isOriginal"`
	IsPrivate   *bool      `json:"isPrivate"`
	CoverURL    *string    `json:"coverUrl"`
	PublishTime *time.Time `json:"publishTime"`
}

// UpdateVideoView handles the corresponding HTTP request.
func (h *Handler) UpdateVideoView(c *gin.Context) {
	req := middlewares.GetBound[UpdateVideoReq](c)

	usernameVal, ok := c.Get("username")
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	username, ok := usernameVal.(string)
	if !ok || username == "" {
		res.FailWithMsg("请先登录", c)

		return
	}

	video, err := h.update.UpdateVideo(c.Request.Context(), username, app.UpdateVideoOpts{
		VideoID:     req.VideoID,
		Title:       req.Title,
		Description: req.Description,
		PartitionID: req.PartitionID,
		TagIDs:      req.TagIDs,
		IsOriginal:  req.IsOriginal,
		IsPrivate:   req.IsPrivate,
		CoverURL:    req.CoverURL,
		PublishTime: req.PublishTime,
	})
	if err != nil {
		if isRecordNotFound(err) {
			res.FailWithMsg("视频不存在或无权限", c)

			return
		}

		res.FailWithMsg(err.Error(), c)
		h.log().Error("update video failed", zap.Error(err), zap.Uint("videoId", req.VideoID))

		return
	}

	res.OkWithData(video, c)
}
