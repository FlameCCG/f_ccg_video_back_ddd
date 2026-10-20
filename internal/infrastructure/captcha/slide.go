package captcha

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/config/site"
	infraconfig "f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/google/uuid"                                 // 生成随机 token
	"github.com/redis/go-redis/v9"                           // Redis 客户端
	"github.com/wenlng/go-captcha-assets/resources/imagesv2" // 背景图资源
	"github.com/wenlng/go-captcha-assets/resources/tiles"    // 滑块模板资源
	"github.com/wenlng/go-captcha/v2/slide"                  // go-captcha 滑块核心库
	"go.uber.org/zap"
)

const (
	defaultSlideCaptchaTTL     = 2 * time.Minute // 默认滑块有效期
	defaultSlideCaptchaPadding = 10              // 默认滑块容错像素
)

type slideCaptchaStorePayload struct {
	Scene string `json:"scene"`

	X      int `json:"x"`      // 正确的 X 坐标
	Y      int `json:"y"`      // 正确的 Y 坐标
	ThumbY int `json:"thumbY"` // 拼图块固定 Y 坐标
}

// SlideCaptcha 滑块验证码适配器（原 service/captcha_service/slide_captcha.go）。
type SlideCaptcha struct {
	store       payloadStore
	cfgProvider *infraconfig.Provider

	once      sync.Once     // 保证生成器只初始化一次
	generator slide.Captcha // 滑块验证码生成器
	initErr   error         // 初始化阶段出现的错误
}

var _ app.SlideCaptchaPort = (*SlideCaptcha)(nil)

// NewSlideCaptcha 构造滑块验证码适配器。
func NewSlideCaptcha(
	redisClient *redis.Client,
	cfgProvider *infraconfig.Provider,
	logger *zap.Logger,
) *SlideCaptcha {
	return &SlideCaptcha{
		store:       payloadStore{redis: redisClient, logger: logger},
		cfgProvider: cfgProvider,
	}
}

// Generate 生成滑块验证码，并把答案写入 Redis。
func (c *SlideCaptcha) Generate(ctx context.Context, scenes ...string) (*app.SlideCaptchaResult, error) {
	scene := "register"
	if len(scenes) > 0 {
		scene = site.ResolveCaptchaScene(scenes[0], scene)
	}

	if err := c.ensure(); err != nil {
		return nil, err
	}

	captData, err := c.generator.Generate()
	if err != nil {
		return nil, err
	}

	token := uuid.NewString()
	block := captData.GetData()
	payload := slideCaptchaStorePayload{Scene: scene, X: block.X, Y: block.Y, ThumbY: block.Y}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	if err = c.store.store(ctx, constants.SlideCaptchaRedisKeyPrefix+token, data, c.ttl(scene)); err != nil {
		return nil, err
	}

	masterB64, err := captData.GetMasterImage().ToBase64()
	if err != nil {
		return nil, err
	}

	tileB64, err := captData.GetTileImage().ToBase64()
	if err != nil {
		return nil, err
	}

	return &app.SlideCaptchaResult{
		MasterImage: masterB64,
		TileImage:   tileB64,
		Token:       token,
		ThumbY:      payload.ThumbY,
	}, nil
}

// Verify 校验拖动落点是否在容错范围内（答案一次性消费）。
func (c *SlideCaptcha) Verify(ctx context.Context, req app.SlideCaptchaVerifyReq, scenes ...string) (bool, error) {
	scene := "register"
	if len(scenes) > 0 {
		scene = site.ResolveCaptchaScene(scenes[0], scene)
	}

	if req.Token == "" {
		return false, ErrCaptchaExpired
	}

	key := constants.SlideCaptchaRedisKeyPrefix + req.Token

	// P1-8: 原子 GETDEL，同一 token 并发仅一方消费成功。
	raw, err := c.store.consume(ctx, key)
	if err != nil {
		return false, err
	}

	var payload slideCaptchaStorePayload

	_ = json.Unmarshal(raw, &payload)
	if payload.Scene != scene {
		return false, ErrCaptchaExpired
	}

	return slide.Validate(req.X, req.Y, payload.X, payload.Y, c.padding(scene)), nil
}

func (c *SlideCaptcha) ensure() error {
	c.once.Do(func() {
		builder := slide.NewBuilder()

		images, err := imagesv2.GetImages()
		if err != nil {
			c.initErr = err

			return
		}

		rawTiles, err := tiles.GetTiles()
		if err != nil {
			c.initErr = err

			return
		}

		graphs := make([]*slide.GraphImage, 0, len(rawTiles))
		for _, graph := range rawTiles {
			graphs = append(graphs, &slide.GraphImage{
				OverlayImage: graph.OverlayImage,
				MaskImage:    graph.MaskImage,
				ShadowImage:  graph.ShadowImage,
			})
		}

		builder.SetResources(
			slide.WithBackgrounds(images),
			slide.WithGraphImages(graphs),
		)

		c.generator = builder.Make()
	})

	return c.initErr
}

func (c *SlideCaptcha) ttl(scenes ...string) time.Duration {
	scene := "register"
	if len(scenes) > 0 {
		scene = scenes[0]
	}
	cfg := c.cfgProvider.Get()
	if cfg == nil {
		return defaultSlideCaptchaTTL
	}
	return time.Duration(cfg.Site.CaptchaSettings(scene, "slide").TTLSeconds) * time.Second
}

func (c *SlideCaptcha) padding(scenes ...string) int {
	scene := "register"
	if len(scenes) > 0 {
		scene = scenes[0]
	}
	cfg := c.cfgProvider.Get()
	if cfg == nil {
		return defaultSlideCaptchaPadding
	}
	return cfg.Site.CaptchaSettings(scene, "slide").Padding
}
