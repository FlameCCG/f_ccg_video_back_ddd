package video

import (
	"errors"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	app "f_ccg_video_back_ddd/internal/application/video"

	"github.com/gin-gonic/gin"
)

// AdminRebuildVectorIndexReq 控制视频向量索引全量回填。
type AdminRebuildVectorIndexReq struct {
	Recreate bool `json:"recreate"`
	Workers  int  `json:"workers" binding:"omitempty,gte=1,lte=16" label:"并发数"`
}

// AdminRebuildVectorIndexView 异步触发全量回填并返回当前状态。
func (h *AdminHandler) AdminRebuildVectorIndexView(c *gin.Context) {
	req := middlewares.GetBound[AdminRebuildVectorIndexReq](c)

	status, err := h.vectors.StartRebuild(c.Request.Context(), app.VectorRebuildOptions{
		Recreate: req.Recreate,
		Workers:  req.Workers,
	})
	if err != nil {
		if errors.Is(err, app.ErrVectorRebuildInProgress) {
			res.FailWithCause(err, clienterror.Message(err), c)

			return
		}

		res.FailWithCause(err, "启动向量索引重建失败: "+clienterror.Message(err), c)

		return
	}

	res.OkWithData(status, c, "向量索引重建已开始，可轮询状态接口查看进度")
}

// AdminVectorIndexStatusView 返回进程内回填任务状态快照。
func (h *AdminHandler) AdminVectorIndexStatusView(c *gin.Context) {
	res.OkWithData(h.vectors.RebuildStatus(), c)
}
