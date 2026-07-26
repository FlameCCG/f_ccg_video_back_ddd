// Package validate provides package functionality.
package validate

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zh_translations "github.com/go-playground/validator/v10/translations/zh"
)

var trans ut.Translator

func init() {
	// 创建翻译器
	uni := ut.New(zh.New())
	trans, _ = uni.GetTranslator("zh")
	// 注册翻译器
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if ok {
		_ = zh_translations.RegisterDefaultTranslations(v, trans)
	}
	//v.RegisterTagNameFunc(func(field reflect.StructField) string {
	//	label := field.Tag.Get("label")
	//	if label == "" {
	//		return field.Name
	//	}
	//	return label
	//})
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		label := field.Tag.Get("label")
		if label == "" {
			label = field.Name
		}

		name := field.Tag.Get("json")

		return fmt.Sprintf("%s---%s", name, label)
	})
}

// ValidateError 导出符号.
func ValidateError(err error) (msg string) {
	var errs validator.ValidationErrors
	if !errors.As(err, &errs) {
		return "请求参数错误"
	}

	var msgList []string

	for _, e := range errs {
		m := e.Translate(trans)
		_list := strings.Split(m, "---")
		msgList = append(msgList, _list[1])
	}

	msg = strings.Join(msgList, ";")

	return msg
}
