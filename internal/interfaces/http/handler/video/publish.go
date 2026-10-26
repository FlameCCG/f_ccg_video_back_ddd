package video

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/res"
	"time"

	app "f_ccg_video_back_ddd/internal/application/video"

	"github.com/gin-gonic/gin"
)

// PublishVideoReq 是 HTTP 投稿载荷，支持单源文件或 Parts；上传是否完成需由服务端重新核验，不能仅信任 FilePath。
type PublishVideoReq struct {
	Title       string           `json:"title" binding:"required,max=200" label:"视频标题"`
	Description string           `json:"description" binding:"max=5000" label:"视频描述"`
	PartitionID uint             `json:"partitionId" binding:"required" label:"分区ID"`
	TagIDs      []uint           `json:"tags" binding:"max=10" label:"标签列表"`
	IsOriginal  bool             `json:"isOriginal" label:"是否原创"`
	IsPrivate   bool             `json:"isPrivate" label:"是否私密"`
	CoverURL    string           `json:"coverUrl" binding:"required" label:"封面图片URL"`
	FilePath    string           `json:"filePath" label:"文件路径"`
	FileName    string           `json:"fileName" binding:"omitempty,max=255" label:"源文件名"`
	FileHash    string           `json:"fileHash" label:"文件hash"`
	Parts       []PartPublishReq `json:"parts" label:"分P列表"`
	PublishTime time.Time        `json:"publishTime" label:"定时发布时间"`
}

// PartPublishReq 描述一次多分P投稿中的源文件与可选标题，数组顺序映射到稿件分P排序。
type PartPublishReq struct {
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

// PublishVideoView 从认证上下文取得作者身份，把请求翻译为 PublishVideoCommand，保存与异步派发由应用服务编排。
func (h *Handler) PublishVideoView(c *gin.Context) {
	req := middlewares.GetBound[PublishVideoReq](c)

	uid, loggedIn := getUID(c)
	if !loggedIn {
		res.FailWithMsg("用户未登录", c)
		return
	}

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
		AuthorID:       uid,
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
		res.FailWithCause(err, clienterror.Message(err), c)

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

// UpdateVideoReq 是部分改稿载荷；指针字段区分缺省和显式零值，TagIDs 的 nil 与空数组分别表示保留和清空。
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

// UpdateVideoView 将当前作者身份和可选修改字段交给改稿用例，不在接口层直接推导视频状态。
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
			res.FailWithCause(err, "视频不存在或无权限", c)

			return
		}

		res.FailWithCause(err, clienterror.Message(err), c)
		res.RecordFailure(apperr.Wrap("handler/video/publish", nil, err), c)

		return
	}

	res.OkWithData(video, c)
}
