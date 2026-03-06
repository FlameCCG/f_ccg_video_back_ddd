package video

import (
	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/ctxutil"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// DanmuSendReq 携带作品/分P与弹幕样式，TimeOffset 是播放器时间轴秒数，发送时间由服务端另行记录。
type DanmuSendReq struct {
	VideoID    uint   `json:"videoId" binding:"required"`
	PartID     uint   `json:"partId" binding:"omitempty,gt=0"`
	Content    string `json:"content" binding:"required,min=1,max=200"`
	TimeOffset int    `json:"timeOffset" binding:"required,min=0"`
	Color      string `json:"color" binding:"omitempty,max=20"`
	FontSize   int8   `json:"fontSize" binding:"omitempty,min=10,max=80"`
	Position   int8   `json:"position" binding:"omitempty,oneof=0 1 2"`
}

// SendDanmuView 发送弹幕（HTTP）.
func (h *Handler) SendDanmuView(c *gin.Context) {
	req := middlewares.GetBound[DanmuSendReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	dto, err := h.danmu.Send(c.Request.Context(), uid, app.SendDanmuParams{
		VideoID:     req.VideoID,
		VideoPartID: req.PartID,
		Content:     req.Content,
		TimeOffset:  req.TimeOffset,
		Color:       req.Color,
		FontSize:    req.FontSize,
		Position:    req.Position,
	})
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithData(dto, c)
}

// DanmuWSReq 描述弹幕房间握手参数；Token 用于前置身份处理，Origin 校验另由升级器负责。
type DanmuWSReq struct {
	VideoID uint   `form:"videoId" binding:"required"`
	PartID  uint   `form:"partId" binding:"omitempty,gt=0"`
	Token   string `form:"token" binding:"omitempty"`
}

// DanmuWSView 弹幕 WebSocket 房间.
func (h *Handler) DanmuWSView(c *gin.Context) {
	req := middlewares.GetBound[DanmuWSReq](c)
	// WebSocket 不套请求级超时；这里只给 upgrade 前的分区解析加短 deadline。
	ctx, cancel := ctxutil.WithTimeout(c.Request.Context(), ctxutil.RequestDefaultTimeout)
	defer cancel()

	partID, err := h.danmu.ResolveDanmuPart(ctx, req.VideoID, req.PartID)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	uid := h.parseUIDFromToken(req.Token, c.GetHeader("Authorization"))
	if err := h.hub.ServeWS(c, req.VideoID, partID, uid); err != nil {
		// Upgrade 阶段失败时才会走到这里，正常情况下连接会由客户端维护
		res.FailWithMsg("连接弹幕服务失败", c)

		return
	}
}

// DanmuHistoryReq 按播放时间区间读取历史弹幕，Start/End 不表示日历发送时间。
type DanmuHistoryReq struct {
	VideoID uint `form:"videoId" binding:"required"`
	PartID  uint `form:"partId" binding:"omitempty,gt=0"`
	Start   int  `form:"start" binding:"omitempty,min=0"`
	End     int  `form:"end" binding:"omitempty,min=0"`
	Limit   int  `form:"limit" binding:"omitempty,min=1,max=500"`
}

// DanmuHistoryView 历史弹幕查询.
func (h *Handler) DanmuHistoryView(c *gin.Context) {
	req := middlewares.GetBound[DanmuHistoryReq](c)

	uid, ok := getUID(c)
	if !ok {
		uid = 0
	}

	list, err := h.danmu.History(c.Request.Context(), app.DanmuHistoryParams{
		VideoID:     req.VideoID,
		VideoPartID: req.PartID,
		UserID:      uid,
		Start:       req.Start,
		End:         req.End,
		Limit:       req.Limit,
	})
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithList(list, int64(len(list)), c)
}

// DanmuRoomListReq 描述房间分页与播放区间/发送日期筛选，业务归一化和可见性由用例检查。
type DanmuRoomListReq struct {
	VideoID  uint   `form:"videoId" binding:"required,gt=0"`
	PartID   uint   `form:"partId" binding:"omitempty,gt=0"`
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"pageSize" binding:"omitempty,min=1,max=5000"`
	Start    int    `form:"start" binding:"omitempty,min=0"` // 播放时间起点（毫秒，含）
	End      int    `form:"end" binding:"omitempty,min=0"`   // 播放时间终点（毫秒，不含）
	Date     string `form:"date" binding:"omitempty"`        // YYYY-MM-DD
}

// DanmuRoomListView 房间弹幕列表；传 start/end 时按播放时间分段返回.
func (h *Handler) DanmuRoomListView(c *gin.Context) {
	req := middlewares.GetBound[DanmuRoomListReq](c)

	uid, ok := getUID(c)
	if !ok {
		uid = 0
	}

	useTimeRange := c.Query("start") != "" || c.Query("end") != ""

	list, total, err := h.danmu.RoomList(c.Request.Context(), app.DanmuRoomListParams{
		VideoID:      req.VideoID,
		VideoPartID:  req.PartID,
		Page:         req.Page,
		PageSize:     req.PageSize,
		Date:         req.Date,
		UserID:       uid,
		Start:        req.Start,
		End:          req.End,
		UseTimeRange: useTimeRange,
	})
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	if useTimeRange {
		res.OkWithData(map[string]any{
			"list":      list,
			"total":     total,
			"returned":  len(list),
			"truncated": int64(len(list)) < total,
			"start":     req.Start,
			"end":       req.End,
		}, c)

		return
	}

	res.OkWithList(list, total, c)
}

// DanmuMonthDatesReq 查询浏览器本地月份里有哪些弹幕。
type DanmuMonthDatesReq struct {
	VideoID uint   `form:"videoId" binding:"required,gt=0"`
	PartID  uint   `form:"partId" binding:"omitempty,gt=0"`
	Month   string `form:"month" binding:"required"`
	Start   string `form:"start" binding:"required"`
	End     string `form:"end" binding:"required"`
}

// DanmuMonthDatesView 返回该时间窗内每条弹幕的发送时刻（库内 Asia/Shanghai）。
func (h *Handler) DanmuMonthDatesView(c *gin.Context) {
	req := middlewares.GetBound[DanmuMonthDatesReq](c)

	result, err := h.danmu.MonthDates(
		c.Request.Context(),
		req.VideoID,
		req.PartID,
		req.Month,
		req.Start,
		req.End,
	)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithData(result, c)
}

// DanmuLikeReq 指定点赞切换的弹幕 ID，点赞用户来自认证上下文。
type DanmuLikeReq struct {
	DanmuID uint `json:"danmuId" binding:"required,gt=0"`
}

// DanmuLikeView 点赞/取消点赞弹幕.
func (h *Handler) DanmuLikeView(c *gin.Context) {
	req := middlewares.GetBound[DanmuLikeReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	result, err := h.danmu.ToggleLike(c.Request.Context(), uid, req.DanmuID)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithData(result, c)
}

// DeleteDanmuReq 指定作者侧删除的单条弹幕，资源归属和计数维护由应用用例执行。
type DeleteDanmuReq struct {
	DanmuID uint `json:"danmuId" binding:"required,gt=0"`
}

// DeleteDanmuView 用户删除自己的弹幕.
func (h *Handler) DeleteDanmuView(c *gin.Context) {
	req := middlewares.GetBound[DeleteDanmuReq](c)

	uid, ok := getUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.danmu.DeleteByUser(c.Request.Context(), uid, req.DanmuID); err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithMsg("删除成功", c)
}

// CreatorDanmuListReq 查询当前创作者相关的弹幕分页，支持排序与内容过滤。
type CreatorDanmuListReq struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"pageSize" binding:"omitempty,min=1,max=50"`
	Sort     int    `form:"sort" binding:"omitempty,oneof=0 1"`  // 0最近 1点赞最多
	Keyword  string `form:"keyword" binding:"omitempty,max=100"` // 按弹幕内容/发送用户/视频标题搜索
}

// CreatorDanmuListView 创作中心：视频弹幕列表（时间倒序）.
func (h *Handler) CreatorDanmuListView(c *gin.Context) {
	req := middlewares.GetBound[CreatorDanmuListReq](c)

	uid, ok := getUID(c)
	if !ok || uid == 0 {
		res.FailWithMsg("未登录", c)

		return
	}

	list, total, err := h.danmu.CreatorList(c.Request.Context(), uid, req.Page, req.PageSize, req.Sort, req.Keyword)
	if err != nil {
		res.FailWithMsg(err.Error(), c)

		return
	}

	res.OkWithList(list, total, c)
}
