// Package markdownUtils 渲染 Markdown 并提取 HTML 文本内容。
//
// MdToHTML 调用 Markdown 渲染器，ExtractContent 再把渲染结果转为文本简介。
// 渲染并不等同于对不可信 HTML 的安全清洗，调用方展示内容时仍需按实际输入与渲染策略处理。
package markdownUtils

import (
	"bytes"

	"github.com/PuerkitoBio/goquery"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// MdToHTML 启用常用 Markdown 扩展并渲染 HTML，链接使用新窗口标记；此函数不执行 HTML 安全清洗。
func MdToHTML(md string) string {
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
// 解析失败时回退原文并保留错误；截取长度以 Unicode 字符计。
func ExtractContent(content string, length int) (string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(MdToHTML(content))))

	plain := content
	if err == nil {
		plain = doc.Text()
	}

	if length < 0 {
		length = 0
	}

	runes := []rune(plain)
	if len(runes) > length {
		runes = runes[:length]
	}

	return string(runes), err
}
