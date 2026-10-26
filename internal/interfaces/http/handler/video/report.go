package video

import (
	"errors"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"
	"f_ccg_video_back_ddd/pkg/uploadcheck"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// maxReportImages 单次举报最多上传的图片数。
const maxReportImages = 5

// 举报图片上传相关错误（文案与原 saveReportImages 逐字一致）。
var (
	errReadReportImages  = errors.New("读取上传图片失败")
	errTooManyImages     = errors.New("最多上传5张图片")
	errStorageConfig     = errors.New("存储配置错误")
	errReadImageFailed   = errors.New("读取图片失败")
	errUploadImageFailed = errors.New("上传图片失败")
)

// ReportReq 携带作品举报目标、原因与补充说明，举报人从认证上下文取得。
type ReportReq struct {
	VideoID uint   `form:"videoId" json:"videoId" binding:"required,gt=0"`
	Reason  string `form:"reason" json:"reason" binding:"required,max=200"`
	Detail  string `form:"detail" json:"detail" binding:"omitempty,max=1000"`
}

// ReportVideoView 举报视频.
func (h *Handler) ReportVideoView(c *gin.Context) {
	req := middlewares.GetBound[ReportReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	images, err := h.saveReportImages(c, "video")
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	report, err := h.reports.CreateVideoReport(
		c.Request.Context(), uid, req.VideoID, req.Reason, req.Detail, images,
	)
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	res.OkWithData(report, c)
}

// DanmuReportReq 携带弹幕举报目标、原因与说明，与举报处理状态分开建模。
type DanmuReportReq struct {
	DanmuID uint   `json:"danmuId" binding:"required,gt=0"`
	Reason  string `json:"reason" binding:"required,max=200"`
	Detail  string `json:"detail" binding:"omitempty,max=1000"`
}

// ReportDanmuView 举报弹幕.
func (h *Handler) ReportDanmuView(c *gin.Context) {
	req := middlewares.GetBound[DanmuReportReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	report, err := h.reports.CreateDanmuReport(c.Request.Context(), uid, req.DanmuID, req.Reason, req.Detail)
	if err != nil {
		res.FailWithCause(err, clienterror.Message(err), c)

		return
	}

	res.OkWithData(report, c)
}

// saveReportImages 保存举报附带的图片并返回可访问 URL 列表。
//
// 非 multipart 请求返回 (nil, nil)；校验与错误文案与原 saveReportImages 逐字一致。
func (h *Handler) saveReportImages(c *gin.Context, reportType string) ([]string, error) {
	middlewares.LimitRequestBody(c, 52<<20)

	form, err := c.MultipartForm()
	if err != nil {
		if errors.Is(err, http.ErrNotMultipart) {
			return nil, nil
		}

		return nil, errReadReportImages
	}

	files := form.File["images"]
	if len(files) == 0 {
		return nil, nil
	}

	if len(files) > maxReportImages {
		return nil, errTooManyImages
	}

	if err := h.uploads.EnsureStorageReady(); err != nil {
		return nil, errStorageConfig
	}

	imageURLs := make([]string, 0, len(files))

	for _, file := range files {
		f, openErr := file.Open()
		if openErr != nil {
			return nil, errReadImageFailed
		}

		ext, contentType, validateErr := uploadcheck.ValidateImage(
			f, file.Filename, file.Size, uploadcheck.DefaultMaxImageBytes,
		)
		if validateErr != nil {
			_ = f.Close()

			return nil, validateErr
		}

		fileName := uuid.NewString() + ext
		relativePath := path.Join("reports", reportType, fileName)

		url, saveErr := h.uploads.SaveImage(c.Request.Context(), relativePath, file.Size, f, contentType)

		_ = f.Close()

		if saveErr != nil {
			return nil, errUploadImageFailed
		}

		imageURLs = append(imageURLs, url)
	}

	return imageURLs, nil
}
