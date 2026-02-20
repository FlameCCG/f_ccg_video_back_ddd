package video

import (
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// HomeFeedReq is an exported API type.
type HomeFeedReq struct {
	Page        int  `form:"page" binding:"omitempty,min=1"`
	PageSize    int  `form:"pageSize" binding:"omitempty,min=1,max=50"`
	PartitionID uint `form:"partitionId" binding:"omitempty,gt=0"`
}

// HomeFeedView 首页推荐流，登录用户优先按历史偏好推荐.
func (h *Handler) HomeFeedView(c *gin.Context) {
	req := middlewares.GetBound[HomeFeedReq](c)
	ctx := c.Request.Context()

	uid := h.resolveUID(c)

	list, total, err := h.feed.HomepageFeed(ctx, uid, req.PartitionID, req.Page, req.PageSize)
	if err != nil {
		h.log().Error("get homepage feed failed", zap.Error(err), zap.Uint("uid", uid))
		res.FailWithMsg("获取首页推荐失败", c)

		return
	}

	res.OkWithList(list, total, c)
}

// VideoHotReq is an exported API type.
type VideoHotReq struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"pageSize" binding:"omitempty,min=1,max=50"`
}

// HotView 综合热门视频列表.
func (h *Handler) HotView(c *gin.Context) {
	req := middlewares.GetBound[VideoHotReq](c)

	list, total, err := h.feed.HotFeed(c.Request.Context(), req.Page, req.PageSize)
	if err != nil {
		res.FailWithMsg("获取综合热门失败", c)

		return
	}

	res.OkWithList(list, total, c)
}

// VideoRankReq is an exported API type.
type VideoRankReq struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"pageSize" binding:"omitempty,min=1,max=50"`
}

// RankView 综合排行榜（Top100）.
func (h *Handler) RankView(c *gin.Context) {
	req := middlewares.GetBound[VideoRankReq](c)

	list, total, err := h.feed.RankTop(c.Request.Context(), req.Page, req.PageSize)
	if err != nil {
		res.FailWithMsg("获取排行榜失败", c)

		return
	}

	res.OkWithList(list, total, c)
}

// VideoRecommendReq is an exported API type.
type VideoRecommendReq struct {
	VideoID uint `form:"videoId" binding:"required,gt=0"`
	Size    int  `form:"size" binding:"omitempty,min=1,max=50"`
}

// RecommendView 视频详情页周边推荐.
func (h *Handler) RecommendView(c *gin.Context) {
	req := middlewares.GetBound[VideoRecommendReq](c)

	list, err := h.recommend.RelatedVideos(c.Request.Context(), req.VideoID, req.Size)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithList(list, int64(len(list)), c)
}

// ListSubmittablePartitionsView 用户投稿分区列表（仅启用且允许投稿）.
func (h *Handler) ListSubmittablePartitionsView(c *gin.Context) {
	list, err := h.partitions.ListSubmittable(c.Request.Context())
	if err != nil {
		res.FailWithMsg("获取失败", c)

		return
	}

	res.OkWithData(list, c)
}
