package identity

import (
	"strings"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// UploadBannerView 上传用户主页横幅图片.
func (h *Handler) UploadBannerView(c *gin.Context) {
	fileHash := strings.TrimSpace(c.PostForm("fileHash"))

	// 与原实现一致：fileHash 校验在读取 multipart 文件之前，
	// 保证非法 hash 回该错误文案而不是「请上传横幅文件」。
	if err := h.banners.ValidateUploadID(fileHash); err != nil {
		res.FailWithMsg(err.Error(), c)

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
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithData(gin.H{
		"bannerUrl": bannerURL,
	}, c)
}
