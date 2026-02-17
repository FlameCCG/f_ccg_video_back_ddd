package captcha

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	app "f_ccg_video_back_ddd/internal/application/identity"
	"f_ccg_video_back_ddd/internal/infrastructure/config"
	"f_ccg_video_back_ddd/pkg/constants"

	"github.com/golang/freetype/truetype"                              // 加载嵌入式中文字体，避免依赖宿主环境
	"github.com/google/uuid"                                           // 生成高随机性的 token，防止被预测
	"github.com/redis/go-redis/v9"                                     // Redis 客户端
	"github.com/wenlng/go-captcha-assets/bindata/chars"                // 提供预置汉字字符集，提升安全性
	"github.com/wenlng/go-captcha-assets/resources/fonts/fzshengsksjw" // 嵌入式中文字体资源
	"github.com/wenlng/go-captcha-assets/resources/imagesv2"           // 背景图素材库，增加干扰项
	"github.com/wenlng/go-captcha/v2/click"                            // go-captcha 点击验证码核心库
	"go.uber.org/zap"
)

const (
	defaultClickCaptchaTTL     = 2 * time.Minute // 配置缺失时的默认有效期
	defaultClickCaptchaPadding = 20              // 配置缺失时的默认容错像素
)

// clickCaptchaTarget 表示答案区域，字段结构用于 Redis 存储。
type clickCaptchaTarget struct {
	Index  int `json:"index"`  // 正确答案顺序索引
	X      int `json:"x"`      // 答案区域左上角 X 坐标
	Y      int `json:"y"`      // 答案区域左上角 Y 坐标
	Width  int `json:"width"`  // 答案区域宽度
	Height int `json:"height"` // 答案区域高度
}

type clickCaptchaStorePayload struct {
	Dots []clickCaptchaTarget `json:"dots"` // 存放全部正确答案
}

// ClickCaptcha 点击验证码适配器。
//
// 生成器由实例字段持有，Wire 注入唯一实例，禁止使用包级可变单例。
type ClickCaptcha struct {
	store       payloadStore
	cfgProvider *config.Provider

	once      sync.Once     // 保证生成器只初始化一次
	generator click.Captcha // 点击验证码生成器实例
	initErr   error         // 记录初始化错误，便于快速返回
}

var _ app.ClickCaptchaPort = (*ClickCaptcha)(nil)

// NewClickCaptcha 构造点击验证码适配器。
func NewClickCaptcha(
	redisClient *redis.Client,
	cfgProvider *config.Provider,
	logger *zap.Logger,
) *ClickCaptcha {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &ClickCaptcha{
		store:       payloadStore{redis: redisClient, logger: logger},
		cfgProvider: cfgProvider,
	}
}

// Generate 生成点击验证码，并把答案写入 Redis。
func (c *ClickCaptcha) Generate(ctx context.Context) (*app.ClickCaptchaResult, error) {
	// 初始化生成器资源
	if err := c.ensure(); err != nil {
		return nil, err
	}

	// 调用 go-captcha 获取主图、提示图与答案
	captData, err := c.generator.Generate()
	if err != nil {
		return nil, err
	}

	token := uuid.NewString()                          // 生成唯一 token 用于关联答案
	targets := buildClickTargets(captData.GetData())   // 将 map[int]*Dot 转为稳定切片
	payload := clickCaptchaStorePayload{Dots: targets} // 构造写入 Redis 的载荷

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// 写入 Redis 并设置 TTL
	if err = c.store.store(ctx, constants.ClickCaptchaRedisKeyPrefix+token, data, c.ttl()); err != nil {
		return nil, err
	}

	masterB64, err := captData.GetMasterImage().ToBase64() // 主图转 base64，供前端展示
	if err != nil {
		return nil, err
	}

	thumbB64, err := captData.GetThumbImage().ToBase64() // 提示图转 base64，指示点击顺序
	if err != nil {
		return nil, err
	}

	return &app.ClickCaptchaResult{ // 返回给前端
		MasterImage: masterB64,
		ThumbImage:  thumbB64,
		Token:       token,
	}, nil
}

// Verify 校验点击顺序与坐标是否落在容错范围内（答案一次性消费）。
func (c *ClickCaptcha) Verify(ctx context.Context, token string, points []app.ClickCaptchaPoint) (bool, error) {
	if token == "" || len(points) == 0 {
		return false, ErrCaptchaExpired
	}

	key := constants.ClickCaptchaRedisKeyPrefix + token

	// 原子 GETDEL 保证同一 token 并发时仅一方消费成功。
	raw, err := c.store.consume(ctx, key)
	if err != nil {
		return false, err
	}

	var payload clickCaptchaStorePayload

	_ = json.Unmarshal(raw, &payload)

	// 数量不匹配说明验证码失效或前端少传点位
	if len(payload.Dots) == 0 || len(payload.Dots) != len(points) {
		return false, ErrCaptchaExpired
	}

	targetMap := make(map[int]clickCaptchaTarget, len(payload.Dots)) // 构建索引映射加速查找
	for _, target := range payload.Dots {
		targetMap[target.Index] = target
	}

	padding := c.padding()
	success := true // 默认校验成功

	for _, point := range points {
		target, ok := targetMap[point.Index] // 查找对应答案
		if !ok {
			success = false

			break
		}

		// 校验点击是否落在容错范围内
		if !click.Validate(point.X, point.Y, target.X, target.Y, target.Width, target.Height, padding) {
			c.store.logger.Error("点击位置不在答案区域内",
				zap.Int("index", point.Index), zap.Int("x", point.X), zap.Int("y", point.Y))

			success = false

			break
		}
	}

	return success, nil
}

func (c *ClickCaptcha) ensure() error {
	c.once.Do(func() { // 懒加载生成器
		builder := click.NewBuilder() // 构造生成器

		font, err := fzshengsksjw.GetFont() // 加载嵌入式中文字体
		if err != nil {
			c.initErr = err

			return
		}

		images, err := imagesv2.GetImages() // 加载背景图资源
		if err != nil {
			c.initErr = err

			return
		}

		builder.SetResources( // 配置字符集、字体与背景，提升安全性
			click.WithChars(chars.GetChineseChars()),
			click.WithFonts([]*truetype.Font{font}),
			click.WithBackgrounds(images),
		)

		c.generator = builder.Make() // 生成最终实例
	})

	return c.initErr
}

func (c *ClickCaptcha) ttl() time.Duration {
	cfg := c.cfgProvider.Get()
	// 读取配置中的秒数
	if cfg != nil && cfg.Site != nil && cfg.Site.Login != nil && cfg.Site.Login.TextClickCaptchaTTL > 0 {
		return time.Duration(cfg.Site.Login.TextClickCaptchaTTL) * time.Second
	}

	return defaultClickCaptchaTTL // 配置缺失时使用默认值
}

func (c *ClickCaptcha) padding() int {
	cfg := c.cfgProvider.Get()
	// 读取配置中的容错像素
	if cfg != nil && cfg.Site != nil && cfg.Site.Login != nil && cfg.Site.Login.TextClickCaptchaPadding > 0 {
		return cfg.Site.Login.TextClickCaptchaPadding
	}

	return defaultClickCaptchaPadding // 配置缺失时使用默认值
}

// buildClickTargets 将目标 map 转成按索引排序的稳定切片。验证码答案不得写入日志或标准输出。
func buildClickTargets(raw map[int]*click.Dot) []clickCaptchaTarget {
	targets := make([]clickCaptchaTarget, 0, len(raw)) // 预分配容量
	for idx, dot := range raw {                        // 遍历所有目标
		targets = append(targets, clickCaptchaTarget{
			Index:  idx,
			X:      dot.X,
			Y:      dot.Y,
			Width:  dot.Width,
			Height: dot.Height,
		})
	}

	sort.Slice(targets, func(i, j int) bool { // 按索引排序，保证顺序稳定
		return targets[i].Index < targets[j].Index
	})

	return targets
}
