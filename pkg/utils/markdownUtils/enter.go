// Package markdownUtils provides package functionality.
package markdownUtils

import (
	"bytes"

	"github.com/PuerkitoBio/goquery"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/sirupsen/logrus"
)

// MdToHtml 导出符号.
func MdToHtml(md string) string {
	// 创建带有扩展的 markdown 解析器
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse([]byte(md))
	// 创建带有扩展的 HTML 渲染器
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return string(markdown.Render(doc, renderer))
}

// ExtractContent 导出符号.
func ExtractContent(content string, length int) (abs string) {
	// 如果不传简介，那么从正文中截取前200个字作为简介
	// 把markdown转成html,再取文本
	toHtml := MdToHtml(content)

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(toHtml)))
	if err != nil {
		logrus.Infof("提取文章内容失败：%v", err)
		// 备用方案：直接从markdown中截取
		plainText := content
		if len(plainText) > length {
			abs = string([]rune(plainText)[:length])
		} else {
			abs = plainText
		}
	} else {
		plainText := doc.Text()
		if len(plainText) > length {
			abs = string([]rune(plainText)[:length])
		} else {
			abs = plainText
		}
	}

	return abs
}
