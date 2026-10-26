package contentops

import (
	"errors"
	"f_ccg_video_back_ddd/internal/config"
	"f_ccg_video_back_ddd/internal/config/site"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"
	"strings"

	app "f_ccg_video_back_ddd/internal/application/contentops"

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
		res.FailWithCause(err, "站点配置未初始化", c)

		return
	}

	res.OkWithData(SiteResp{Site: siteCfg}, c)
}

// AdminConfigItemView 管理员站点配置（按名称）.
// Query: name.
func (h *Handler) AdminConfigItemView(c *gin.Context) {
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		res.FailWithMsg("配置项名称不能为空", c)

		return
	}

	resp, err := h.siteCfg.AdminConfigByName(name)
	if err != nil {
		res.FailWithCause(err, "不支持的配置项", c)

		return
	}

	res.OkWithData(resp, c)
}

// AdminUpdateConfigItemView 管理员更新站点配置（按名称）.
// Query: name.
func (h *Handler) AdminUpdateConfigItemView(c *gin.Context) {
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		res.FailWithMsg("配置项名称不能为空", c)

		return
	}

	var err error

	restartRequired := false

	middlewares.LimitRequestBody(c, middlewares.DefaultJSONBodyLimit)

	// 绑定停留在 interfaces 层；应用层负责浅拷贝顶层、保留密钥、原子发布并写回文件。
	switch name {
	case configNameSite:
		err = h.bindSiteSection(c)
	case app.ConfigNameLogger:
		var logger *config.Logger
		if err = c.ShouldBindJSON(&logger); err == nil {
			restartRequired = h.siteCfg.LoggerRestartRequired(logger)
			err = h.siteCfg.UpdateLogger(logger)
		}
	case app.ConfigNameEmail:
		err = bindConfigSection(c, h.siteCfg.UpdateEmail)
	case app.ConfigNameAI:
		err = bindConfigSection(c, h.siteCfg.UpdateAI)
	case app.ConfigNameTranscode:
		var body struct {
			config.Transcode
			LocalConsumerDisabled *bool `json:"localConsumerDisabled"`
		}
		if err = c.ShouldBindJSON(&body); err == nil {
			err = h.siteCfg.UpdateTranscode(&body.Transcode, body.LocalConsumerDisabled)
		}
	case configNameThirdLogin:
		err = bindConfigSection(c, h.siteCfg.UpdateThirdLogin)
	case app.ConfigNameJWT:
		err = bindConfigSection(c, h.siteCfg.UpdateJWT)
	case configNameServer:
		var body AdminServerCORSConfig
		if err = c.ShouldBindJSON(&body); err == nil {
			err = h.siteCfg.UpdateServerCORS(body.CorsOrigins)
		}
	default:
		res.FailWithMsg("不支持的配置项", c)

		return
	}

	finishHandlerAdminUpdateConfigItemView(c, name, err, restartRequired)
}

// respondConfigUpdateError 把配置更新失败映射为与原实现逐字一致的响应。
//
// 读/写配置文件失败回显固定文案；请求体绑定失败与配置校验失败走 res.FailWithError
// （与原实现一致，经 validate 转换后为「请求参数错误」或校验详情）。
func respondConfigUpdateError(err error, c *gin.Context) {
	switch {
	case errors.Is(err, app.ErrConfigFileRead):
		res.FailWithCause(err, "读取配置文件失败", c)
	case errors.Is(err, app.ErrConfigFileWrite):
		res.FailWithCause(err, "更新配置文件失败", c)
	case errors.Is(err, app.ErrConfigItemUnsupported):
		res.FailWithMsg("不支持的配置项", c)
	default:
		res.FailWithError(err, c)
	}
}

func finishHandlerAdminUpdateConfigItemView(c *gin.Context, name string, err error, restartRequired bool) {
	if err != nil {
		respondConfigUpdateError(err, c)

		return
	}

	if name == app.ConfigNameLogger {
		res.OkWithData(gin.H{"restartRequired": restartRequired}, c, "更新成功")
		return
	}

	res.OkWithMsg("更新成功", c)
}

func bindConfigSection[T any](c *gin.Context, update func(*T) error) error {
	var value *T
	if err := c.ShouldBindJSON(&value); err != nil {
		return err
	}

	return update(value)
}

func (h *Handler) bindSiteSection(c *gin.Context) error {
	var body *struct {
		site.Site
		DisableUserSubmission *bool `json:"disableUserSubmission"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		return err
	}

	if body == nil {
		return h.siteCfg.UpdateSite(nil, nil)
	}

	return h.siteCfg.UpdateSite(&body.Site, body.DisableUserSubmission)
}
