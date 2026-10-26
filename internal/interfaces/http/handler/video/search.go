package video

import (
	app "f_ccg_video_back_ddd/internal/application/video"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	// homeHotKeywordLimit 首页热搜关键词条数。
	homeHotKeywordLimit = 10
	// defaultSuggestSize 搜索联想默认条数。
	defaultSuggestSize = 10
)

// 视频搜索排序枚举（取值与 search 上下文一致，属外部契约）。
const (
	// videoSortRelevance 0：按 ES _score 相关性排序。
	videoSortRelevance = 0
	// videoSortDuration 3：按时长排序（枚举上界）。
	videoSortDuration = 3
	// videoSortViews 1：按播放量排序（未传或非法值的默认）。
	videoSortViews = 1
	// sortOrderDesc 0：降序。
	sortOrderDesc = 0
	// sortOrderAsc 1：升序。
	sortOrderAsc = 1
)

// HomeSearchTopView 首页热搜关键词前十.
func (h *Handler) HomeSearchTopView(c *gin.Context) {
	list, _, err := h.search.HotKeywords(c.Request.Context(), 1, homeHotKeywordLimit)
	if err != nil {
		res.FailWithMsg("获取首页热搜关键词失败", c)

		return
	}

	res.OkWithData(list, c)
}

// SearchReq 描述关键词、分页和排序选项，应用/领域层将外部排序值归一化。
type SearchReq struct {
	Keyword    string `form:"keyword" binding:"required"`
	Page       int    `form:"page" binding:"omitempty,min=1"`
	PageSize   int    `form:"pageSize" binding:"omitempty,min=1,max=100"`
	VideoSort  int    `form:"videoSort" binding:"omitempty,oneof=0 1 2 3"` // 0相关性(_score) 1播放量 2弹幕数 3时长
	VideoOrder int    `form:"videoOrder" binding:"omitempty,oneof=0 1"`    // 0降序 1升序
}

// VideoSearchView 把视频搜索条件交给应用搜索能力，并将结果转换为当前视频接口响应。
func (h *Handler) VideoSearchView(c *gin.Context) {
	req := middlewares.GetBound[SearchReq](c)

	page := req.Page
	if page <= 0 {
		page = 1
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	ctx := c.Request.Context()

	data, err := h.search.SearchVideos(ctx, app.SearchOptions{
		Keyword:    req.Keyword,
		Page:       page,
		PageSize:   pageSize,
		VideoSort:  normalizeVideoSort(req.VideoSort),
		VideoOrder: normalizeOrder(req.VideoOrder),
	})
	if err != nil {
		res.FailWithMsg("搜索失败", c)

		return
	}

	h.search.RecordHotKeyword(ctx, req.Keyword)
	// Tags 不再返回
	res.OkWithData(data, c)
}

// SuggestReq 以 Prefix 和期望条数请求搜索联想，具体缓存与回退策略留在搜索用例。
type SuggestReq struct {
	Prefix string `form:"prefix" binding:"required"`
	Size   int    `form:"size" binding:"omitempty,min=1,max=20"`
}

// SuggestView 将前缀与条数交给搜索联想用例，不在 Handler 构造 Elasticsearch DSL。
func (h *Handler) SuggestView(c *gin.Context) {
	req := middlewares.GetBound[SuggestReq](c)

	size := req.Size
	if size <= 0 {
		size = defaultSuggestSize
	}

	suggestions, err := h.search.Suggest(c.Request.Context(), req.Prefix, size)
	if err != nil {
		res.FailWithMsg("获取搜索提示失败", c)

		return
	}

	res.OkWithData(suggestions, c)
}

// normalizeVideoSort 归一化视频搜索排序字段；未传或非法值默认按播放量降序。
func normalizeVideoSort(sort int) int {
	if sort < videoSortRelevance || sort > videoSortDuration {
		return videoSortViews
	}

	return sort
}

// normalizeOrder 归一化排序方向；仅显式传 1 时升序，其余都按降序处理。
func normalizeOrder(order int) int {
	if order == sortOrderAsc {
		return sortOrderAsc
	}

	return sortOrderDesc
}
