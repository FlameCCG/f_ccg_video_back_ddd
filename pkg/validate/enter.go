// Package validate 为 Gin 请求校验配置并提供中文错误翻译。
//
// New 显式配置 validator 并构造 Translator，Middleware 将翻译器放入当前请求上下文。
// 结构体 label 用于可读字段名，json 标签用于字段定位，ErrorFromContext 在翻译器缺失时返回通用错误。
// 组合根负责初始化，无需依赖包级 init；字段格式校验通过后仍要执行领域业务规则。
package validate

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zhtranslations "github.com/go-playground/validator/v10/translations/zh"
)

const ginTranslatorKey = "request_validation_translator"

// Translator 持有请求参数校验错误的中文翻译器。
// 实例由组合根注入，不通过 init 或包级可变单例装配。
type Translator struct {
	translator ut.Translator
}

// New 构造翻译器，并显式配置 Gin 使用的 validator 实例。
func New() *Translator {
	universal := ut.New(zh.New())
	translator, _ := universal.GetTranslator("zh")

	if engine, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = zhtranslations.RegisterDefaultTranslations(engine, translator)
		engine.RegisterTagNameFunc(func(field reflect.StructField) string {
			label := field.Tag.Get("label")
			if label == "" {
				label = field.Name
			}

			return fmt.Sprintf("%s---%s", field.Tag.Get("json"), label)
		})
	}

	return &Translator{translator: translator}
}

// Middleware 把显式注入的翻译器放入当前 Gin 请求作用域。
func (translator *Translator) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ginTranslatorKey, translator)
		c.Next()
	}
}

// Error 返回与原 ValidateError 相同的中文校验错误文本。
func (translator *Translator) Error(err error) string {
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) || translator == nil || translator.translator == nil {
		return "请求参数错误"
	}

	messages := make([]string, 0, len(validationErrors))
	for _, fieldError := range validationErrors {
		translated := fieldError.Translate(translator.translator)
		parts := strings.SplitN(translated, "---", 2)
		if len(parts) == 2 {
			messages = append(messages, parts[1])

			continue
		}
		messages = append(messages, translated)
	}

	return strings.Join(messages, ";")
}

// ErrorFromContext 使用当前请求中由 Router 注入的翻译器。
func ErrorFromContext(c *gin.Context, err error) string {
	if c == nil {
		return "请求参数错误"
	}

	value, ok := c.Get(ginTranslatorKey)
	if !ok {
		return "请求参数错误"
	}

	translator, ok := value.(*Translator)
	if !ok {
		return "请求参数错误"
	}

	return translator.Error(err)
}
