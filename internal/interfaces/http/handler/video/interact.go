package video

import (
	"errors"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/apperr"
	"f_ccg_video_back_ddd/pkg/res"

	domain "f_ccg_video_back_ddd/internal/domain/video"

	"github.com/gin-gonic/gin"
)

// ViewReq 指定需要增加播放计数的作品，当前实现的播放计数不按用户去重。
type ViewReq struct {
	VideoID uint `json:"videoId" binding:"required"`
}

// ViewView 把作品与可选用户身份传给播放用例，返回更新后的播放计数。
func (h *Handler) ViewView(c *gin.Context) {
	req := middlewares.GetBound[ViewReq](c)

	uid, ok := getUID(c)
	if !ok {
		uid = 0
	}

	stats, err := h.interact.View(c.Request.Context(), req.VideoID, uid)
	if err != nil {
		if isRecordNotFound(err) {
			res.FailWithCause(err, "视频不存在或未发布", c)

			return
		}

		res.RecordFailure(apperr.Wrap("handler/video/interact", nil, err), c)
		res.FailWithCause(err, "增加播放量失败", c)

		return
	}

	res.OkWithData(map[string]int64{"views": stats.Views}, c)
}

// LikeReq 指定点赞切换目标，登录用户作为关系写入主体。
type LikeReq struct {
	VideoID uint `json:"videoId" binding:"required"`
}

// LikeView 调用视频点赞切换用例并返回切换状态，不直接修改 Redis 或关系表。
func (h *Handler) LikeView(c *gin.Context) {
	req := middlewares.GetBound[LikeReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	stats, err := h.interact.ToggleLike(c.Request.Context(), req.VideoID, uid)
	if err != nil {
		if isRecordNotFound(err) {
			res.FailWithCause(err, "视频不存在或未发布", c)

			return
		}

		res.RecordFailure(apperr.Wrap("handler/video/interact", nil, err), c)
		res.FailWithCause(err, "操作失败", c)

		return
	}

	res.OkWithData(map[string]any{
		"likes": stats.Likes,
	}, c)
}

// CoinReq 指定作品与投币数量，额度、余额、重复投币和作者奖励由应用/领域规则裁定。
type CoinReq struct {
	VideoID uint `json:"videoId" binding:"required"`
	Coins   int  `json:"coins" binding:"omitempty,min=1,max=2"` // 默认 1，最多 2
}

// CoinView 把当前用户与投币参数交给事务用例，账本和展示计数分别由应用层协调。
func (h *Handler) CoinView(c *gin.Context) {
	req := middlewares.GetBound[CoinReq](c)
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
			res.FailWithCause(err, "对本稿件的投币次数已用完", c)

			return
		}

		if h.interact.IsCoinInsufficient(err) {
			res.FailWithCause(err, "硬币余额不足", c)

			return
		}

		if isRecordNotFound(err) {
			res.FailWithCause(err, "视频不存在或未发布", c)

			return
		}

		res.RecordFailure(apperr.Wrap("handler/video/interact", nil, err), c)
		res.FailWithCause(err, "投币失败", c)

		return
	}

	res.OkWithData(map[string]any{
		"coinCount": stats.CoinCount,
	}, c)
}

// FavoriteReq 指定收藏切换目标；FolderID=0 使用当前用户默认收藏夹。
type FavoriteReq struct {
	VideoID  uint `json:"videoId" binding:"required"`
	FolderID uint `json:"folderId" binding:"omitempty"`
}

// FavoriteView 调用收藏切换用例并返回当前计数，用户多收藏夹去重由用例处理。
func (h *Handler) FavoriteView(c *gin.Context) {
	req := middlewares.GetBound[FavoriteReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	stats, err := h.interact.ToggleFavorite(c.Request.Context(), req.VideoID, uid, req.FolderID)
	if err != nil {
		if isRecordNotFound(err) {
			res.FailWithCause(err, "视频不存在或收藏夹不存在", c)

			return
		}

		res.RecordFailure(apperr.Wrap("handler/video/interact", nil, err), c)
		res.FailWithCause(err, "收藏操作失败", c)

		return
	}

	res.OkWithData(map[string]any{
		"favoriteCount": stats.FavoriteCount,
	}, c)
}

// TripleReq 组合点赞、投币与收藏的操作意图，不能在 Handler 中拆成三个互不协调的独立写入。
type TripleReq struct {
	VideoID  uint `json:"videoId" binding:"required"`
	FolderID uint `json:"folderId" binding:"omitempty"`
	Coins    int  `json:"coins" binding:"omitempty,min=1,max=2"`
}

// TripleView 一键三连.
func (h *Handler) TripleView(c *gin.Context) {
	req := middlewares.GetBound[TripleReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	result, err := h.interact.Triple(c.Request.Context(), req.VideoID, uid, req.FolderID, int64(req.Coins))
	if err != nil {
		if h.interact.IsCoinInsufficient(err) {
			res.FailWithCause(err, "硬币余额不足", c)

			return
		}

		if isRecordNotFound(err) {
			res.FailWithCause(err, "视频不存在或未发布", c)

			return
		}

		res.RecordFailure(apperr.Wrap("handler/video/interact", nil, err), c)
		res.FailWithCause(err, "操作失败", c)

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
