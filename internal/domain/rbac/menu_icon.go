package rbac

import (
	"errors"
	"regexp"
	"strings"
)

// menuIconMaxBytes 菜单图标最大字节数（与 AI 生成路径的长度限制一致）。
const menuIconMaxBytes = 2048

// menuIconEventAttribute 匹配 SVG/HTML 内联事件属性（onload、onclick 等）。
var menuIconEventAttribute = regexp.MustCompile(`(?i)\son[a-z]+\s*=`)

// 菜单图标相关错误。
var (
	// ErrMenuIconUnsafe 菜单图标包含不安全内容（script / foreignObject / javascript: / 事件属性）.
	ErrMenuIconUnsafe = errors.New("菜单图标包含不安全内容")
	// ErrMenuIconTooLong 菜单图标超出长度限制.
	ErrMenuIconTooLong = errors.New("菜单图标超出长度限制（最多 2048 字节）")
)

// ValidateMenuIcon 校验菜单图标能否安全渲染。
//
// 拒绝 script / foreignObject / javascript: 与内联事件属性（防存储型 XSS），
// 并限制长度；空串视为合法（菜单可以不设置图标）。创建/更新菜单的手工路径
// 与 AI 生成路径共用该校验（见缺陷台账 FIND-019）。
func ValidateMenuIcon(icon string) error {
	if len(icon) > menuIconMaxBytes {
		return ErrMenuIconTooLong
	}

	lower := strings.ToLower(icon)
	if strings.Contains(lower, "<script") ||
		strings.Contains(lower, "<foreignobject") ||
		strings.Contains(lower, "javascript:") ||
		menuIconEventAttribute.MatchString(icon) {
		return ErrMenuIconUnsafe
	}

	return nil
}
