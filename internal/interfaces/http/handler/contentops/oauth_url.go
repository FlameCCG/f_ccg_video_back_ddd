package contentops

import (
	app "f_ccg_video_back_ddd/internal/application/contentops"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// GitHubURLReq 请求/响应结构体.
type GitHubURLReq struct {
	State               string `form:"state"`
	CodeChallenge       string `form:"codeChallenge"`
	CodeChallengeMethod string `form:"codeChallengeMethod" binding:"omitempty,oneof=S256"`
}

// LinuxDoURLReq 请求/响应结构体.
type LinuxDoURLReq struct {
	State string `form:"state"`
}

// XURLReq 请求/响应结构体.
type XURLReq struct {
	State               string `form:"state"`
	CodeChallenge       string `form:"codeChallenge" binding:"required"`
	CodeChallengeMethod string `form:"codeChallengeMethod" binding:"omitempty,oneof=S256 plain"`
}

// QQUrlView 处理对应 HTTP 请求.
func (h *Handler) QQUrlView(c *gin.Context) {
	authURL, err := h.oauthURL.QQURL(c.Request.Context())
	if err != nil {
		res.FailWithMsg("OAuth authorization unavailable", c)
		return
	}
	res.OkWithData(authURL, c)
}

// GoogleUrlView 处理对应 HTTP 请求.
func (h *Handler) GoogleUrlView(c *gin.Context) {
	authURL, err := h.oauthURL.GoogleURL(c.Request.Context(), c.Query("state"))
	if err != nil {
		res.FailWithMsg("生成授权链接失败", c)

		return
	}

	res.OkWithData(authURL, c)
}

// GitHubUrlView 处理对应 HTTP 请求.
func (h *Handler) GitHubUrlView(c *gin.Context) {
	req := middlewares.GetBound[GitHubURLReq](c)

	authURL, err := h.oauthURL.GitHubURL(c.Request.Context(), app.GitHubURLParams{
		State:               req.State,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
	})
	if err != nil {
		res.FailWithMsg("获取失败", c)

		return
	}

	res.OkWithData(authURL, c)
}

// LinuxDoUrlView 处理对应 HTTP 请求.
func (h *Handler) LinuxDoUrlView(c *gin.Context) {
	req := middlewares.GetBound[LinuxDoURLReq](c)

	authURL, err := h.oauthURL.LinuxDoURL(c.Request.Context(), req.State)
	if err != nil {
		res.FailWithMsg("获取失败", c)

		return
	}

	res.OkWithData(authURL, c)
}

// XUrlView 处理对应 HTTP 请求.
func (h *Handler) XUrlView(c *gin.Context) {
	req := middlewares.GetBound[XURLReq](c)

	authURL, err := h.oauthURL.XURL(c.Request.Context(), app.XURLParams{
		State:               req.State,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
	})
	if err != nil {
		res.FailWithMsg("获取失败", c)

		return
	}

	res.OkWithData(authURL, c)
}
