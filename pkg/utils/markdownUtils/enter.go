// Package markdownUtils 渲染 Markdown 并提取 HTML 文本内容。
//
// MdToHtml 调用 Markdown 渲染器，ExtractContent 再把渲染结果转为文本简介。
// 渲染并不等同于对不可信 HTML 的安全清洗，调用方展示内容时仍需按实际输入与渲染策略处理。
package markdownUtils

import (
	"bytes"

	"github.com/PuerkitoBio/goquery"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/sirupsen/logrus"
)

// MdToHtml 启用常用 Markdown 扩展并渲染 HTML，链接使用新窗口标记；此函数不执行 HTML 安全清洗。
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

// ExtractContent 先把 Markdown 渲染为 HTML，再提取纯文本并按 length 截取简介。
// 解析失败时回退原文；当前实现以字节长度判阈、以 rune 截取，调用方需注意多字节文本的长度边界。
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
