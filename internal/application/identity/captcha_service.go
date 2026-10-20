package identity

import (
	"context"

	"f_ccg_video_back_ddd/internal/application/shared"
	"f_ccg_video_back_ddd/internal/config/site"
)

// ClickCaptchaResult 点击验证码生成结果（字段与原 ClickCaptchaGenerateResp 一致）。
type ClickCaptchaResult struct {
	MasterImage string // 主图 base64，展示背景及所有候选文字
	ThumbImage  string // 提示图 base64，指示需要点击的文字顺序
	Token       string // 唯一 token，用于在 Redis 中索引答案
}

// ClickCaptchaPoint 用户点击点位。
type ClickCaptchaPoint struct {
	Index int `json:"index"` // 点击顺序索引
	X     int `json:"x"`     // 点击时的 X 坐标
	Y     int `json:"y"`     // 点击时的 Y 坐标
}

// SlideCaptchaResult 滑块验证码生成结果（字段与原 SlideCaptchaGenerateResp 一致）。
type SlideCaptchaResult struct {
	MasterImage string // 背景图 base64
	TileImage   string // 拼图块 base64
	Token       string // 答案 token
	ThumbY      int    // 拼图块固定 Y 坐标
}

// SlideCaptchaVerifyReq 滑块验证请求。
type SlideCaptchaVerifyReq struct {
	Token string `json:"token"` // 答案 token
	X     int    `json:"x"`     // 拖动后的 X 坐标
	Y     int    `json:"y"`     // 拖动后的 Y 坐标
}

// ClickCaptchaPort 点击验证码端口（实现在 infrastructure/captcha）。
type ClickCaptchaPort interface {
	Generate(ctx context.Context, scenes ...string) (*ClickCaptchaResult, error)
	Verify(ctx context.Context, token string, points []ClickCaptchaPoint, scenes ...string) (bool, error)
}

// SlideCaptchaPort 滑块验证码端口（实现在 infrastructure/captcha）。
type SlideCaptchaPort interface {
	Generate(ctx context.Context, scenes ...string) (*SlideCaptchaResult, error)
	Verify(ctx context.Context, req SlideCaptchaVerifyReq, scenes ...string) (bool, error)
}

// GraphicCaptchaStore 图形（字符）验证码存储端口。
//
// 方法集与 base64Captcha.Store 完全一致，因此可直接交给
// base64Captcha.NewCaptcha 使用；实现由 infrastructure/captcha 提供，
// 取代原 global.CaptchaStore 包级单例。
type GraphicCaptchaStore interface {
	Set(id string, value string) error
	Get(id string, clear bool) string
	Verify(id, answer string, clear bool) bool
}

// CaptchaService 验证码用例服务（原 service/captcha_service 的对外入口）。
//
// 图形/点击/滑块三种验证码的开关来自站点配置，支持运行期热更新。
type CaptchaService struct {
	click       ClickCaptchaPort
	slide       SlideCaptchaPort
	graphic     GraphicCaptchaStore
	emails      EmailCaptchaStore
	cfgProvider shared.ConfigReader
}

// NewCaptchaService 构造验证码用例服务。
func NewCaptchaService(
	click ClickCaptchaPort,
	slide SlideCaptchaPort,
	graphic GraphicCaptchaStore,
	emails EmailCaptchaStore,
	cfgProvider shared.ConfigReader,
) *CaptchaService {
	return &CaptchaService{
		click:       click,
		slide:       slide,
		graphic:     graphic,
		emails:      emails,
		cfgProvider: cfgProvider,
	}
}

// GenerateClick 生成点击验证码。
func (s *CaptchaService) GenerateClick(ctx context.Context, scenes ...string) (*ClickCaptchaResult, error) {
	return s.click.Generate(ctx, scenes...)
}

// VerifyClick 校验点击验证码。
func (s *CaptchaService) VerifyClick(ctx context.Context, token string, points []ClickCaptchaPoint, scenes ...string) (bool, error) {
	return s.click.Verify(ctx, token, points, scenes...)
}

// GenerateSlide 生成滑块验证码。
func (s *CaptchaService) GenerateSlide(ctx context.Context, scenes ...string) (*SlideCaptchaResult, error) {
	return s.slide.Generate(ctx, scenes...)
}

// VerifySlide 校验滑块验证码。
func (s *CaptchaService) VerifySlide(ctx context.Context, req SlideCaptchaVerifyReq, scenes ...string) (bool, error) {
	return s.slide.Verify(ctx, req, scenes...)
}

// GraphicStore 返回图形验证码存储，供接口层构造 base64Captcha 实例。
func (s *CaptchaService) GraphicStore() GraphicCaptchaStore {
	return s.graphic
}

// VerifyGraphic 校验图形验证码（校验后清除）。
func (s *CaptchaService) VerifyGraphic(id, answer string) bool {
	return s.graphic.Verify(id, answer, true)
}

// InteractiveCaptchaOptions 读取当前场景的交互验证码开关。
func (s *CaptchaService) InteractiveCaptchaOptions(scene string) site.CaptchaOptions {
	cfg := s.cfgProvider.Get()
	if cfg == nil {
		return site.CaptchaOptions{}
	}
	return cfg.Site.InteractiveCaptchaOptions(scene)
}

// ClickCaptchaEnabled 登录是否启用点击验证码。
func (s *CaptchaService) ClickCaptchaEnabled() bool {
	cfg := s.cfgProvider.Get()

	return cfg != nil && cfg.Site != nil && cfg.Site.Login != nil && cfg.Site.Login.TextClickCaptcha
}

// SlideCaptchaEnabled 管理员登录是否启用滑块验证码。
func (s *CaptchaService) SlideCaptchaEnabled() bool {
	cfg := s.cfgProvider.Get()

	return cfg != nil && cfg.Site != nil && cfg.Site.Login != nil && cfg.Site.Login.SlideCaptcha
}

// GraphicCaptchaEnabled 注册是否启用图形验证码。
func (s *CaptchaService) GraphicCaptchaEnabled() bool {
	cfg := s.cfgProvider.Get()

	return cfg != nil && cfg.Site != nil && cfg.Site.Register != nil && cfg.Site.Register.TextGraphicCaptcha
}

// EmailCaptchaEnabled 注册是否启用邮箱验证码。
func (s *CaptchaService) EmailCaptchaEnabled() bool {
	cfg := s.cfgProvider.Get()

	return cfg != nil && cfg.Site != nil && cfg.Site.Register != nil && cfg.Site.Register.EmailCaptcha
}
