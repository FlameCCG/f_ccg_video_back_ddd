package identity

import (
	"encoding/json"
	"f_ccg_video_back_ddd/internal/interfaces/clienterror"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// UpdateUserConfReq 请求/响应结构体.
type UpdateUserConfReq struct {
	OpenCollect   *bool     `json:"openCollect" label:"是否公开收藏"`
	OpenFans      *bool     `json:"openFans" label:"是否公开粉丝"`
	OpenFollow    *bool     `json:"openFollow" label:"是否公开关注"`
	OpenLikeVideo *bool     `json:"openLikeVideo" label:"是否公开点赞视频"`
	OpenCoinVideo *bool     `json:"openCoinVideo" label:"是否公开投币视频"`
	BannerID      *uint     `json:"bannerId" binding:"omitempty,gt=0" label:"系统默认横幅ID"`
	BannerURL     string    `json:"bannerUrl" label:"自定义横幅URL"`
	LikeTags      *[]string `json:"likeTags" label:"喜欢的标签"`
}

// UserConfRes 是用户配置响应体。
//
// LikeTags 使用 json.RawMessage 承载数据库 JSON，原样输出且 nil 输出 null，
// 避免 []byte 被编码为 base64；user_id 与关联对象不参与 JSON 序列化。
type UserConfRes struct {
	LikeTags      json.RawMessage `json:"likeTags"`
	BannerID      uint            `json:"bannerId"`
	BannerURL     string          `json:"bannerUrl"`
	OpenCollect   bool            `json:"openCollect"`
	OpenFans      bool            `json:"openFans"`
	OpenFollow    bool            `json:"openFollow"`
	OpenLikeVideo bool            `json:"openLikeVideo"`
	OpenCoinVideo bool            `json:"openCoinVideo"`
}

// GetUserConfView 获取用户配置.
func (h *Handler) GetUserConfView(c *gin.Context) {
	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	view, err := h.confs.View(c.Request.Context(), uid)
	if err != nil {
		res.FailWithMsg("用户配置不存在", c)

		return
	}

	res.OkWithData(UserConfRes{
		LikeTags:      view.LikeTags,
		BannerID:      view.BannerID,
		BannerURL:     view.BannerURL,
		OpenCollect:   view.OpenCollect,
		OpenFans:      view.OpenFans,
		OpenFollow:    view.OpenFollow,
		OpenLikeVideo: view.OpenLikeVideo,
		OpenCoinVideo: view.OpenCoinVideo,
	}, c)
}

// UpdateUserConfView 更新用户配置.
func (h *Handler) UpdateUserConfView(c *gin.Context) {
	req := middlewares.GetBound[UpdateUserConfReq](c)

	uid, ok := middlewares.GetUID(c)
	if !ok {
		res.FailWithMsg("请先登录", c)

		return
	}

	if err := h.confs.Update(c.Request.Context(), app.UpdateUserConfInput{
		UserID:        uid,
		OpenCollect:   req.OpenCollect,
		OpenFans:      req.OpenFans,
		OpenFollow:    req.OpenFollow,
		OpenLikeVideo: req.OpenLikeVideo,
		OpenCoinVideo: req.OpenCoinVideo,
		BannerID:      req.BannerID,
		BannerURL:     req.BannerURL,
		LikeTags:      req.LikeTags,
	}); err != nil {
		res.FailWithMsg(clienterror.Message(err), c)

		return
	}

	res.OkWithMsg("更新成功", c)
}
