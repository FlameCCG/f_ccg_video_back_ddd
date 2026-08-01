package video

import (
	"context"
	"path"
	"strings"

	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"f_ccg_video_back_ddd/pkg/res"
	"f_ccg_video_back_ddd/pkg/uploadcheck"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// UploadChunkReq is an exported API type.
type UploadChunkReq struct {
	FileHash string `json:"fileHash" form:"fileHash" binding:"required"` // 整个文件内容哈希（用于目录/断点续传）
	Index    string `json:"index" form:"index" binding:"required"`       // 当前分片索引（从 0 开始）
}

// UploadChunkView handles the corresponding HTTP request.
func (h *Handler) UploadChunkView(c *gin.Context) {
	req := middlewares.GetBound[UploadChunkReq](c)

	file, header, err := c.Request.FormFile("chunk")
	if err != nil {
		h.log().Error("upload chunk failed", zap.Error(err))
		res.FailWithMsg("获取上传分片失败", c)

		return
	}

	defer func() { _ = file.Close() }()

	if err := h.uploads.UploadChunk(c.Request.Context(), req.FileHash, req.Index, header.Size, file); err != nil {
		h.log().Error("upload chunk failed", zap.Error(err))
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithMsg("分片上传成功", c)
}

// ChunkStatusReq is an exported API type.
type ChunkStatusReq struct {
	FileHash string `form:"fileHash" binding:"required"` // 文件 hash (identifier)
}

// ChunkStatusResp is an exported API type.
type ChunkStatusResp struct {
	FileHash       string   `json:"fileHash"`       // 当前文件标识，便于前端直接用于投稿
	Completed      bool     `json:"completed"`      // 是否已完成上传，可用于秒传
	FilePath       string   `json:"filePath"`       // 已完成时的最终文件路径
	UploadedChunks []string `json:"uploadedChunks"` // 已上传的分片索引列表
}

// ChunkStatusView 检查文件分片上传状态.
func (h *Handler) ChunkStatusView(c *gin.Context) {
	req := middlewares.GetBound[ChunkStatusReq](c)

	status, err := h.uploads.GetUploadStatus(c.Request.Context(), req.FileHash)
	if err != nil {
		h.log().Error("list uploaded chunks failed", zap.Error(err))
		res.FailWithMsg("检查上传状态失败", c)

		return
	}

	res.OkWithData(ChunkStatusResp{
		FileHash:       status.FileHash,
		Completed:      status.Completed,
		FilePath:       status.FilePath,
		UploadedChunks: status.UploadedChunks,
	}, c)
}

// CompleteUploadReq is an exported API type.
type CompleteUploadReq struct {
	FileHash    string `json:"fileHash" binding:"required"`
	FileName    string `json:"fileName" binding:"required"`
	TotalChunks int    `json:"totalChunks" binding:"required"`
}

// CompleteUploadResp is an exported API type.
type CompleteUploadResp struct {
	FilePath string `json:"filePath"`
}

// CompleteUploadView handles the corresponding HTTP request.
func (h *Handler) CompleteUploadView(c *gin.Context) {
	req := middlewares.GetBound[CompleteUploadReq](c)

	// 合并分片已完成鉴权与参数绑定，允许脱离客户端断开继续收尾，但必须保留上传级超时。
	mergeCtx, cancel := context.WithTimeout(
		context.WithoutCancel(c.Request.Context()),
		ctxutil.RequestUploadTimeout,
	)
	defer cancel()

	finalPath, err := h.uploads.CompleteUpload(mergeCtx, req.FileHash, req.FileName, req.TotalChunks)
	if err != nil {
		h.log().Error("complete upload failed", zap.Error(err))
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithData(CompleteUploadResp{
		FilePath: finalPath,
	}, c)
}

// UploadImageView 接收前端上传的图片并返回存储地址.
func (h *Handler) UploadImageView(c *gin.Context) {
	fileHash := strings.TrimSpace(c.PostForm("fileHash"))
	if err := h.uploads.ValidateUploadID(fileHash); err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	file, header, err := c.Request.FormFile("cover")
	if err != nil {
		res.FailWithMsg("请上传封面文件", c)

		return
	}

	defer func() { _ = file.Close() }()

	ext, contentType, err := uploadcheck.ValidateImage(
		file, header.Filename, header.Size, uploadcheck.DefaultMaxImageBytes,
	)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	relativePath := buildCoverObjectPath(fileHash, ext)

	imageURL, err := h.uploads.SaveImage(c.Request.Context(), relativePath, header.Size, file, contentType)
	if err != nil {
		h.log().Error("save image failed", zap.Error(err))
		res.FailWithMsg("封面上传失败", c)

		return
	}

	res.OkWithData(gin.H{
		"imageUrl": imageURL,
	}, c)
}

// buildCoverObjectPath 生成封面对象路径（与原 api/video_api/upload_cover.go 逐字一致）。
func buildCoverObjectPath(fileHash, ext string) string {
	ext = strings.TrimSpace(strings.ToLower(ext))
	if ext == "" {
		ext = ".jpg"
	}

	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	return path.Join("images", strings.TrimSpace(fileHash)+"-"+uuid.NewString()+ext)
}
