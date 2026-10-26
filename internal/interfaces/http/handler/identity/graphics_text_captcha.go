package identity

import (
	"crypto/rand"
	"encoding/hex"
	"f_ccg_video_back_ddd/internal/interfaces/http/middlewares"
	"f_ccg_video_back_ddd/pkg/res"
	"fmt"
	"image/color"
	"math/big"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mojocn/base64Captcha"
)

// 图形验证码画布尺寸与噪声参数（与原实现逐字一致）。
const (
	graphicCaptchaHeight    = 80
	graphicCaptchaWidth     = 240
	graphicCaptchaNoiseText = 1
	graphicCaptchaLength    = 5
	graphicCaptchaFont      = "wqy-microhei.ttc"
)

// 图形验证码字符集（与原实现逐字一致）。
const (
	graphicCaptchaAlphanumericSource = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	graphicCaptchaDigitSource        = "0123456789"
	graphicCaptchaLetterSource       = "ABCDEFGHJKLMNPQRSTUVWXYZ"
)

// GraphicsCaptchaRequest 请求/响应结构体.
type GraphicsCaptchaRequest struct {
	Type int `form:"type"` // 验证码类型，1：算术运算 2：数字字母混合 3：纯数字 4：纯字母
}

// GraphicsCaptchaResponse 请求/响应结构体.
type GraphicsCaptchaResponse struct {
	CaptchaID  string `json:"captchaID"`
	CaptchaB64 string `json:"captchaB64"`
}

// GraphicsTextCaptchaView 处理对应 HTTP 请求.
func (h *CaptchaHandler) GraphicsTextCaptchaView(c *gin.Context) {
	req := middlewares.GetBound[GraphicsCaptchaRequest](c)

	// Only rendering uses the library. IDs and answers use the OS CSPRNG,
	// independently of the math/rand state used for decorative image noise.
	id, question, answer, err := newGraphicChallenge(req.Type)
	if err != nil {
		res.FailWithCause(err, "验证码生成失败", c)
		return
	}

	image, err := newGraphicCaptchaDriver(req.Type).DrawCaptcha(question)
	if err != nil {
		res.FailWithCause(err, "验证码生成失败", c)
		return
	}

	if err := h.captcha.GraphicStore().Set(id, answer); err != nil {
		res.FailWithCause(err, "验证码生成失败", c)
		return
	}

	res.OkWithData(GraphicsCaptchaResponse{
		CaptchaID:  id,
		CaptchaB64: image.EncodeB64string(),
	}, c)
}

func newGraphicChallenge(captchaType int) (id, question, answer string, err error) {
	var rawID [32]byte
	if _, err = rand.Read(rawID[:]); err != nil {
		return "", "", "", err
	}

	id = hex.EncodeToString(rawID[:])
	source := ""

	switch captchaType {
	case 2:
		source = graphicCaptchaAlphanumericSource
	case 3:
		source = graphicCaptchaDigitSource
	case 4:
		source = graphicCaptchaLetterSource
	}

	if source != "" {
		var answerSb88 strings.Builder

		for range graphicCaptchaLength {
			index, err := rand.Int(rand.Reader, big.NewInt(int64(len(source))))
			if err != nil {
				return "", "", "", err
			}

			answerSb88.WriteString(string(source[index.Int64()]))
		}

		answer += answerSb88.String()

		return id, answer, answer, nil
	}

	a, err := rand.Int(rand.Reader, big.NewInt(20))
	if err != nil {
		return "", "", "", err
	}

	b, err := rand.Int(rand.Reader, big.NewInt(20))
	if err != nil {
		return "", "", "", err
	}

	op, err := rand.Int(rand.Reader, big.NewInt(3))
	if err != nil {
		return "", "", "", err
	}

	operator, result := "+", a.Int64()+b.Int64()
	if op.Int64() == 1 {
		operator, result = "-", a.Int64()-b.Int64()
	}

	if op.Int64() == 2 {
		operator, result = "x", a.Int64()*b.Int64()
	}

	return id, fmt.Sprintf("%d%s%d=?", a.Int64(), operator, b.Int64()), strconv.FormatInt(result, 10), nil
}

// newGraphicCaptchaDriver 按请求类型构造 base64Captcha 驱动。
//
// 未知类型回落算术验证码，与原 switch 的 default 分支一致。
func newGraphicCaptchaDriver(captchaType int) base64Captcha.Driver {
	// 公共配置：纯白背景 + 指定字体。
	bgColor := &color.RGBA{R: 255, G: 255, B: 255, A: 255}

	switch captchaType {
	case 2:
		// 数字字母混合验证码
		return newGraphicStringDriver(graphicCaptchaAlphanumericSource, bgColor)
	case 3:
		// 纯数字验证码
		return newGraphicStringDriver(graphicCaptchaDigitSource, bgColor)
	case 4:
		// 纯字母验证码
		return newGraphicStringDriver(graphicCaptchaLetterSource, bgColor)
	default:
		// 算术验证码（如 3+5=?）；type=1 与未知类型走同一分支。
		return base64Captcha.NewDriverMath(
			graphicCaptchaHeight, graphicCaptchaWidth, graphicCaptchaNoiseText,
			base64Captcha.OptionShowSineLine|base64Captcha.OptionShowHollowLine,
			bgColor, nil, []string{graphicCaptchaFont},
		)
	}
}

// newGraphicStringDriver 构造字符型图形验证码驱动。
func newGraphicStringDriver(source string, bgColor *color.RGBA) base64Captcha.Driver {
	return base64Captcha.NewDriverString(
		graphicCaptchaHeight, graphicCaptchaWidth, graphicCaptchaNoiseText,
		base64Captcha.OptionShowSineLine|base64Captcha.OptionShowSlimeLine,
		graphicCaptchaLength, source, bgColor, nil, []string{graphicCaptchaFont},
	)
}
