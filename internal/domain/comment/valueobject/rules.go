package valueobject

import (
	"strings"

	commenterrors "f_ccg_video_back_ddd/internal/domain/comment/domainerr"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

const (
	// MaxContentRunes 评论文字长度上限（按 rune 计）。
	MaxContentRunes = 1000
	// DefaultPageSize 列表默认每页数量。
	DefaultPageSize = 20
	// MaxPageSize 前台列表每页数量上限。
	MaxPageSize = 50
	// MaxAdminPageSize 管理端列表每页数量上限。
	MaxAdminPageSize = 100
)

// ResolveTargetType 校验评论对象并推导评论类型。
//
// 规则：videoId 与 dynamicId
// 必须恰好给出一个，否则返回 ErrTargetRequired。
func ResolveTargetType(videoID, dynamicID uint) (enum.CommentType, error) {
	if (videoID > 0 && dynamicID > 0) || (videoID == 0 && dynamicID == 0) {
		return "", commenterrors.ErrTargetRequired
	}

	if videoID > 0 {
		return enum.VideoCommentType, nil
	}

	return enum.DynamicCommentType, nil
}

// NormalizeContent 去除评论文字首尾空白。
func NormalizeContent(content string) string {
	return strings.TrimSpace(content)
}

// ValidateContent 校验规范化后的评论文字。
//
// 规则：文字为空且无图片时报空；文字超过 MaxContentRunes 时报过长。
func ValidateContent(content string, pictureCount int) error {
	if content == "" && pictureCount == 0 {
		return commenterrors.ErrContentEmpty
	}

	if len([]rune(content)) > MaxContentRunes {
		return commenterrors.ErrContentTooLong
	}

	return nil
}

// NormalizeAtUserIDs 规范化 @ 名单，剔除 0 与自己并去重。
//
// 返回候选集还需经应用层查询端口校验关注关系，仅互相关注的用户允许被 @。
func NormalizeAtUserIDs(actorID uint, atUserIDs []uint) []uint {
	if actorID == 0 || len(atUserIDs) == 0 {
		return nil
	}

	unique := make(map[uint]struct{}, len(atUserIDs))
	ids := make([]uint, 0, len(atUserIDs))

	for _, id := range atUserIDs {
		if id == 0 || id == actorID {
			continue
		}

		if _, ok := unique[id]; ok {
			continue
		}

		unique[id] = struct{}{}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return nil
	}

	return ids
}

// NormalizePaging 纠偏前台列表分页参数（每页上限 MaxPageSize）。
func NormalizePaging(page, pageSize int) (int, int) {
	return normalizePaging(page, pageSize, MaxPageSize)
}

// NormalizeAdminPaging 纠偏管理端列表分页参数（每页上限 MaxAdminPageSize）。
func NormalizeAdminPaging(page, pageSize int) (int, int) {
	return normalizePaging(page, pageSize, MaxAdminPageSize)
}

func normalizePaging(page, pageSize, maxPageSize int) (int, int) {
	if page < 1 {
		page = 1
	}

	if pageSize < 1 || pageSize > maxPageSize {
		pageSize = DefaultPageSize
	}

	return page, pageSize
}
