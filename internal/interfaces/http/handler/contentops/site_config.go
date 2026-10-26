package contentops

import (
	"errors"
	"strings"

	app "f_ccg_video_back_ddd/internal/application/contentops"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"

	"github.com/gin-gonic/gin"
)

// 配置项名称（= 查询参数 name），取自应用层常量，避免两处字面量漂移。
const (
	configNameSite       = app.ConfigNameSite
	configNameThirdLogin = app.ConfigNameThirdLogin
	configNameServer     = app.ConfigNameServer
)

// AdminServerCORSConfig 管理端仅暴露 corsOrigins，避免误改监听地址等进程级字段。
type AdminServerCORSConfig struct {
	CorsOrigins []string `json:"corsOrigins"`
}

// SiteResp 请求/响应结构体.
type SiteResp struct {
	Site site.Site `json:"site"`
}

// CommonConfigView 公开站点配置（普通用户）.
func (h *Handler) CommonConfigView(c *gin.Context) {
	siteCfg, err := h.siteCfg.PublicSite()
	if err != nil {
		res.FailWithMsg("站点配置未初始化", c)

		return
	}

	res.OkWithData(SiteResp{Site: siteCfg}, c)
}

// AdminConfigItemView 管理员站点配置（按名称）.
// Query: name
func (h *Handler) AdminConfigItemView(c *gin.Context) {
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		res.FailWithMsg("配置项名称不能为空", c)

		return
	}

	resp, err := h.siteCfg.AdminConfigByName(name)
	if err != nil {
		res.FailWithMsg("不支持的配置项", c)

		return
	}

	res.OkWithData(resp, c)
}

// AdminUpdateConfigItemView 管理员更新站点配置（按名称）.
// Query: name
func (h *Handler) AdminUpdateConfigItemView(c *gin.Context) {
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		res.FailWithMsg("配置项名称不能为空", c)

		return
	}

	var err error

	middlewares.LimitRequestBody(c, middlewares.DefaultJSONBodyLimit)

	// 绑定停留在 interfaces 层；应用层串行保留密钥、校验、写回文件，成功后发布快照。
	switch name {
	case configNameSite:
		var siteCfg *struct {
			site.Site
			DisableUserSubmission *bool `json:"disableUserSubmission"`
		}
		if err = c.ShouldBindJSON(&siteCfg); err == nil {
			if siteCfg == nil {
				err = h.siteCfg.UpdateSite(nil, nil)
			} else {
				err = h.siteCfg.UpdateSite(&siteCfg.Site, siteCfg.DisableUserSubmission)
			}
		}
	case app.ConfigNameLogger:
		var logger *config.Logger
		if err = c.ShouldBindJSON(&logger); err == nil {
			err = h.siteCfg.UpdateLogger(logger)
		}
	case app.ConfigNameEmail:
		var email *config.Email
		if err = c.ShouldBindJSON(&email); err == nil {
			err = h.siteCfg.UpdateEmail(email)
		}
	case app.ConfigNameAI:
		var ai *config.AI
		if err = c.ShouldBindJSON(&ai); err == nil {
			err = h.siteCfg.UpdateAI(ai)
		}
	case app.ConfigNameTranscode:
		var body struct {
			config.Transcode
			LocalConsumerDisabled *bool `json:"localConsumerDisabled"`
		}
		if err = c.ShouldBindJSON(&body); err == nil {
			err = h.siteCfg.UpdateTranscode(&body.Transcode, body.LocalConsumerDisabled)
		}
	case configNameThirdLogin:
		var thirdLogin *config.ThirdLogin
		if err = c.ShouldBindJSON(&thirdLogin); err == nil {
			err = h.siteCfg.UpdateThirdLogin(thirdLogin)
		}
	case app.ConfigNameJWT:
		var jwt *config.Jwt
		if err = c.ShouldBindJSON(&jwt); err == nil {
			err = h.siteCfg.UpdateJWT(jwt)
		}
	case configNameServer:
		var body AdminServerCORSConfig
		if err = c.ShouldBindJSON(&body); err == nil {
			err = h.siteCfg.UpdateServerCORS(body.CorsOrigins)
		}
	default:
		res.FailWithMsg("不支持的配置项", c)

		return
	}

	if err != nil {
		respondConfigUpdateError(err, c)

		return
	}

	res.OkWithMsg("更新成功", c)
}

// respondConfigUpdateError 把配置更新失败映射为与原实现逐字一致的响应。
//
// 读/写配置文件失败回显固定文案；请求体绑定失败与配置校验失败走 res.FailWithError
// （与原实现一致，经 validate 转换后为「请求参数错误」或校验详情）。
func respondConfigUpdateError(err error, c *gin.Context) {
	switch {
	case errors.Is(err, app.ErrConfigFileRead):
		res.FailWithMsg("读取配置文件失败", c)
	case errors.Is(err, app.ErrConfigFileWrite):
		res.FailWithMsg("更新配置文件失败", c)
	case errors.Is(err, app.ErrConfigItemUnsupported):
		res.FailWithMsg("不支持的配置项", c)
	default:
		res.FailWithError(err, c)
	}
}
