package video

import (
	"context"
	"errors"
	"path"
	"strings"

	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"f_ccg_video_back_ddd/pkg/res"
	"f_ccg_video_back_ddd/pkg/uploadcheck"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// UploadChunkReq 标识整文件哈希和从 0 开始的分片序号，二进制内容由 multipart 的 chunk 字段传入。
type UploadChunkReq struct {
	FileHash string `json:"fileHash" form:"fileHash" binding:"required"` // 整个文件内容哈希（用于目录/断点续传）
	Index    string `json:"index" form:"index" binding:"required"`       // 当前分片索引（从 0 开始）
}

// UploadChunkView 打开 multipart 分片流并交给带用户额度校验的上传用例，返回前关闭上传流。
func (h *Handler) UploadChunkView(c *gin.Context) {
	req := middlewares.GetBound[UploadChunkReq](c)
	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	file, header, err := c.Request.FormFile("chunk")
	if err != nil {
		h.log().Error("upload chunk failed", zap.Error(err))
		res.FailWithMsg("获取上传分片失败", c)

		return
	}

	defer func() { _ = file.Close() }()

	if err := h.uploads.UploadChunkForUser(c.Request.Context(), uid, req.FileHash, req.Index, header.Size, file); err != nil {
		h.log().Error("upload chunk failed", zap.Error(err))
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithMsg("分片上传成功", c)
}

// ChunkStatusReq 按文件哈希查询续传状态，查询已完成对象仍需应用层探测存储。
type ChunkStatusReq struct {
	FileHash string `form:"fileHash" binding:"required"` // 文件 hash (identifier)
}

// ChunkStatusResp 同时表达秒传完成状态与已上传分片；Completed=false 时不能把空 FilePath 当成已完成源文件。
type ChunkStatusResp struct {
	FileHash       string   `json:"fileHash"`       // 当前文件标识，便于前端直接用于投稿
	Completed      bool     `json:"completed"`      // 是否已完成上传，可用于秒传
	FilePath       string   `json:"filePath"`       // 已完成时的最终文件路径
	UploadedChunks []string `json:"uploadedChunks"` // 已上传的分片索引列表
}

// ChunkStatusView 检查文件分片上传状态.
func (h *Handler) ChunkStatusView(c *gin.Context) {
	req := middlewares.GetBound[ChunkStatusReq](c)
	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	status, err := h.uploads.GetUploadStatusForUser(c.Request.Context(), uid, req.FileHash)
	if err != nil {
		h.log().Error("list uploaded chunks failed", zap.Error(err))
		if errors.Is(err, app.ErrDailyUploadLimit) || errors.Is(err, app.ErrUploadQuotaUnavailable) {
			res.FailWithMsg(err.Error(), c)
		} else {
			res.FailWithMsg("检查上传状态失败", c)
		}

		return
	}

	res.OkWithData(ChunkStatusResp{
		FileHash:       status.FileHash,
		Completed:      status.Completed,
		FilePath:       status.FilePath,
		UploadedChunks: status.UploadedChunks,
	}, c)
}

// CompleteUploadReq 提供合并目标的哈希、原文件名和预期分片总数，完整性核对由上传用例执行。
type CompleteUploadReq struct {
	FileHash    string `json:"fileHash" binding:"required"`
	FileName    string `json:"fileName" binding:"required"`
	TotalChunks int    `json:"totalChunks" binding:"required"`
}

// CompleteUploadResp 返回合并后可用于投稿的存储路径，仍不表示稿件已写入或媒体已转码。
type CompleteUploadResp struct {
	FilePath string `json:"filePath"`
}

// CompleteUploadView 调用完整性检查与分片合并，认证后允许脱离客户端取消继续收尾，但保留独立上传超时。
func (h *Handler) CompleteUploadView(c *gin.Context) {
	req := middlewares.GetBound[CompleteUploadReq](c)
	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)
		return
	}

	// 合并分片已完成鉴权与参数绑定，允许脱离客户端断开继续收尾，但必须保留上传级超时。
	mergeCtx, cancel := context.WithTimeout(
		context.WithoutCancel(c.Request.Context()),
		ctxutil.RequestUploadTimeout,
	)
	defer cancel()

	finalPath, err := h.uploads.CompleteUploadForUser(mergeCtx, uid, req.FileHash, req.FileName, req.TotalChunks)
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
