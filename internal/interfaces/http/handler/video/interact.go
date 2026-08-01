package video

import (
	"errors"

	domain "f_ccg_video_back_ddd/internal/domain/video"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// VideoViewReq is an exported API type.
type VideoViewReq struct {
	VideoID uint `json:"videoId" binding:"required"`
}

// ViewView handles the corresponding HTTP request.
func (h *Handler) ViewView(c *gin.Context) {
	req := middlewares.GetBound[VideoViewReq](c)

	uid, ok := getUID(c)
	if !ok {
		uid = 0
	}

	stats, err := h.interact.View(c.Request.Context(), req.VideoID, uid)
	if err != nil {
		if isRecordNotFound(err) {
			res.FailWithMsg("视频不存在或未发布", c)

			return
		}

		h.log().Error("view video failed", zap.Error(err), zap.Uint("videoId", req.VideoID))
		res.FailWithMsg("增加播放量失败", c)

		return
	}

	res.OkWithData(map[string]int64{"views": stats.Views}, c)
}

// VideoLikeReq is an exported API type.
type VideoLikeReq struct {
	VideoID uint `json:"videoId" binding:"required"`
}

// LikeView handles the corresponding HTTP request.
func (h *Handler) LikeView(c *gin.Context) {
	req := middlewares.GetBound[VideoLikeReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	stats, err := h.interact.ToggleLike(c.Request.Context(), req.VideoID, uid)
	if err != nil {
		if isRecordNotFound(err) {
			res.FailWithMsg("视频不存在或未发布", c)

			return
		}

		h.log().Error("like toggle failed", zap.Error(err), zap.Uint("videoId", req.VideoID))
		res.FailWithMsg("操作失败", c)

		return
	}

	res.OkWithData(map[string]any{
		"likes": stats.Likes,
	}, c)
}

// VideoCoinReq is an exported API type.
type VideoCoinReq struct {
	VideoID uint `json:"videoId" binding:"required"`
	Coins   int  `json:"coins" binding:"omitempty,min=1,max=2"` // 默认 1，最多 2
}

// CoinView handles the corresponding HTTP request.
func (h *Handler) CoinView(c *gin.Context) {
	req := middlewares.GetBound[VideoCoinReq](c)
	if req.Coins == 0 {
		res.FailWithMsg("至少投一个币哦", c)

		return
	}

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	stats, err := h.interact.Coin(c.Request.Context(), req.VideoID, uid, int64(req.Coins))
	if err != nil {
		if errors.Is(err, domain.ErrCoinLimit) {
			res.FailWithMsg("对本稿件的投币次数已用完", c)

			return
		}

		if h.interact.IsCoinInsufficient(err) {
			res.FailWithMsg("硬币余额不足", c)

			return
		}

		if isRecordNotFound(err) {
			res.FailWithMsg("视频不存在或未发布", c)

			return
		}

		h.log().Error("coin video failed", zap.Error(err), zap.Uint("videoId", req.VideoID))
		res.FailWithMsg("投币失败", c)

		return
	}

	res.OkWithData(map[string]any{
		"coinCount": stats.CoinCount,
	}, c)
}

// VideoFavoriteReq is an exported API type.
type VideoFavoriteReq struct {
	VideoID  uint `json:"videoId" binding:"required"`
	FolderID uint `json:"folderId" binding:"omitempty"`
}

// FavoriteView handles the corresponding HTTP request.
func (h *Handler) FavoriteView(c *gin.Context) {
	req := middlewares.GetBound[VideoFavoriteReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	stats, err := h.interact.ToggleFavorite(c.Request.Context(), req.VideoID, uid, req.FolderID)
	if err != nil {
		if isRecordNotFound(err) {
			res.FailWithMsg("视频不存在或收藏夹不存在", c)

			return
		}

		h.log().Error("favorite toggle failed", zap.Error(err), zap.Uint("videoId", req.VideoID))
		res.FailWithMsg("收藏操作失败", c)

		return
	}

	res.OkWithData(map[string]any{
		"favoriteCount": stats.FavoriteCount,
	}, c)
}

// VideoTripleReq is an exported API type.
type VideoTripleReq struct {
	VideoID  uint `json:"videoId" binding:"required"`
	FolderID uint `json:"folderId" binding:"omitempty"`
	Coins    int  `json:"coins" binding:"omitempty,min=1,max=2"`
}

// TripleView 一键三连.
func (h *Handler) TripleView(c *gin.Context) {
	req := middlewares.GetBound[VideoTripleReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	result, err := h.interact.Triple(c.Request.Context(), req.VideoID, uid, req.FolderID, int64(req.Coins))
	if err != nil {
		if h.interact.IsCoinInsufficient(err) {
			res.FailWithMsg("硬币余额不足", c)

			return
		}

		if isRecordNotFound(err) {
			res.FailWithMsg("视频不存在或未发布", c)

			return
		}

		h.log().Error("triple failed", zap.Error(err), zap.Uint("videoId", req.VideoID))
		res.FailWithMsg("操作失败", c)

		return
	}

	res.OkWithData(map[string]any{
		"likes":         result.Likes,
		"coinCount":     result.CoinCount,
		"favoriteCount": result.FavoriteCount,
		"isLiked":       result.IsLiked,
		"isFavorited":   result.IsFavorited,
		"isCoined":      result.IsCoined,
	}, c)
}
