package dynamic

import "strings"

const (
	// LikeNoticeTitle 动态被点赞时的通知标题。
	LikeNoticeTitle = "有人点赞了你的动态"
	// LikeNoticeFallbackContent 动态正文为空时的通知正文兜底。
	LikeNoticeFallbackContent = "你的动态"
	// LikeNoticeMaxRunes 通知正文摘要的最大字符数，超出部分以省略号结尾。
	LikeNoticeMaxRunes = 80
)

// ShouldNotifyLike 作者存在且点赞者不是作者本人时才发送点赞通知。
func (d *Dynamic) ShouldNotifyLike(actionUserID uint) bool {
	return d != nil && d.userID != 0 && d.userID != actionUserID
}

// LikeNoticeContent 由动态正文生成点赞通知的正文摘要，不修改动态本身。
//
// 规则：裁剪首尾空白；
// 空正文回落为「你的动态」；超过 LikeNoticeMaxRunes 个字符时截断并追加 "..."。
func (d *Dynamic) LikeNoticeContent() string {
	if d == nil {
		return LikeNoticeFallbackContent
	}

	content := strings.TrimSpace(d.content)
	if content == "" {
		return LikeNoticeFallbackContent
	}

	runes := []rune(content)
	if len(runes) > LikeNoticeMaxRunes {
		return string(runes[:LikeNoticeMaxRunes]) + "..."
	}

	return content
}
