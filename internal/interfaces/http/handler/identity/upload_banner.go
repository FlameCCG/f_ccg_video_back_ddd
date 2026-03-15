package identity

import (
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"strings"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// UploadBannerView 上传用户主页横幅图片.
func (h *Handler) UploadBannerView(c *gin.Context) {
	middlewares.LimitRequestBody(c, 12<<20)
	fileHash := strings.TrimSpace(c.PostForm("fileHash"))

	// 读取 multipart 文件前先校验 fileHash，使非法 hash 返回对应错误文案。
	if err := h.banners.ValidateUploadID(fileHash); err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	file, header, err := c.Request.FormFile("banner")
	if err != nil {
		res.FailWithMsg("请上传横幅文件", c)

		return
	}

	defer func() { _ = file.Close() }()

	bannerURL, err := h.banners.UploadBanner(c.Request.Context(), app.UploadBannerInput{
		FileHash: fileHash,
		File:     file,
		Filename: header.Filename,
		Size:     header.Size,
	})
	if err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithData(gin.H{
		"bannerUrl": bannerURL,
	}, c)
}
