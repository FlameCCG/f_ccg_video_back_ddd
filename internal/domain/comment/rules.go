package comment

import (
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
	"strings"
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
// 规则与原 CommentService.Create / List 一致：videoId 与 dynamicId
// 必须恰好给出一个，否则返回 ErrTargetRequired。
func ResolveTargetType(videoID, dynamicID uint) (enum.CommentType, error) {
	if (videoID > 0 && dynamicID > 0) || (videoID == 0 && dynamicID == 0) {
		return "", ErrTargetRequired
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
// 规则与原实现一致：文字为空且无图片时报空；文字超过 MaxContentRunes 时报过长。
func ValidateContent(content string, pictureCount int) error {
	if content == "" && pictureCount == 0 {
		return ErrContentEmpty
	}

	if len([]rune(content)) > MaxContentRunes {
		return ErrContentTooLong
	}

	return nil
}

// NormalizeDraft 规范化发布草稿：校验登录态、评论对象与内容，并推导评论类型。
//
// 校验顺序与原 CommentService.Create 完全一致（登录 → 对象 → 内容 → 长度），
// 保证同一份非法请求返回同一条错误文案。
func NormalizeDraft(draft Draft) (Draft, enum.CommentType, error) {
	if draft.UserID == 0 {
		return draft, "", ErrUserNotLogin
	}

	commentType, err := ResolveTargetType(draft.VideoID, draft.DynamicID)
	if err != nil {
		return draft, "", err
	}

	draft.Content = NormalizeContent(draft.Content)
	if err := ValidateContent(draft.Content, len(draft.Pictures)); err != nil {
		return draft, "", err
	}

	return draft, commentType, nil
}

// HasParent 报告草稿是否为回复（父评论 ID 有效）。
func HasParent(parentID *uint) bool {
	return parentID != nil && *parentID > 0
}

// ResolveRootID 依据父评论推导根评论 ID。
//
// 规则与原实现一致：父评论本身是根评论（RootID 为空）时取父评论 ID，
// 否则沿用父评论的 RootID，使层级恒定为两级。
func ResolveRootID(parentID uint, parent ParentBrief) *uint {
	if parent.RootID != nil {
		return parent.RootID
	}

	id := parentID

	return &id
}

// DeleteDecision 删除评论的授权判定结果。
type DeleteDecision int

const (
	// DeleteAllowed 操作者是评论作者，直接放行。
	DeleteAllowed DeleteDecision = iota
	// DeleteNeedsVideoAuthorCheck 需进一步确认操作者是否为视频作者。
	DeleteNeedsVideoAuthorCheck
	// DeleteNeedsDynamicAuthorCheck 需进一步确认操作者是否为动态作者。
	DeleteNeedsDynamicAuthorCheck
	// DeleteDenied 评论既未关联视频也未关联动态，无人可代删。
	DeleteDenied
)

// DecideDelete 判定删除评论所需的下一步校验。
//
// 规则与原 CommentService.Delete 的分支完全一致：
// 评论作者直接放行；否则按评论所属对象交由视频作者 / 动态作者校验；
// 两者皆无则拒绝。
func DecideDelete(c *Comment, operatorID uint) DeleteDecision {
	if c == nil {
		return DeleteDenied
	}

	if c.UserID == operatorID {
		return DeleteAllowed
	}

	switch {
	case c.VideoID > 0:
		return DeleteNeedsVideoAuthorCheck
	case c.DynamicID > 0:
		return DeleteNeedsDynamicAuthorCheck
	default:
		return DeleteDenied
	}
}

// ValidatePinTarget 校验置顶目标：仅一级评论可置顶，且必须关联视频或动态。
func ValidatePinTarget(c *Comment) error {
	if c == nil {
		return ErrCommentNotFound
	}

	if !c.IsRoot() {
		return ErrPinRootOnly
	}

	if c.VideoID == 0 && c.DynamicID == 0 {
		return ErrPinTargetMissing
	}

	return nil
}

// NormalizeAtUserIDs 规范化 @ 名单：剔除 0 与自己并去重。
//
// 返回的候选集还需经关系校验（仅互相关注可 @），该校验属读侧投影，
// 由应用层经查询端口完成，与原 filterAllowedAtUsers 的两段逻辑一一对应。
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
