package comment

import (
	"strings"

	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Normalize 规范化发布草稿，按登录态、评论对象、内容、长度的顺序校验，并推导评论类型；同一非法请求按此顺序返回错误。
func (draft Draft) Normalize() (Draft, enum.CommentType, error) {
	if draft.UserID == 0 {
		return draft, "", ErrUserNotLogin
	}

	commentType, err := draft.TargetType()
	if err != nil {
		return draft, "", err
	}

	draft.Content = draft.NormalizeContent()
	if err := draft.ValidateContent(); err != nil {
		return draft, "", err
	}

	return draft, commentType, nil
}

// TargetType 校验评论对象并推导评论类型。
//
// 规则：videoId 与 dynamicId
// 必须恰好给出一个，否则返回 ErrTargetRequired。
func (draft Draft) TargetType() (enum.CommentType, error) {
	if (draft.VideoID > 0 && draft.DynamicID > 0) || (draft.VideoID == 0 && draft.DynamicID == 0) {
		return "", ErrTargetRequired
	}

	if draft.VideoID > 0 {
		return enum.VideoCommentType, nil
	}

	return enum.DynamicCommentType, nil
}

// NormalizeContent 去除评论文字首尾空白。
func (draft Draft) NormalizeContent() string {
	return strings.TrimSpace(draft.Content)
}

// ValidateContent 校验规范化后的评论文字。
//
// 规则：文字为空且无图片时报空；文字超过 MaxContentRunes 时报过长。
func (draft Draft) ValidateContent() error {
	if draft.Content == "" && len(draft.Pictures) == 0 {
		return ErrContentEmpty
	}

	if len([]rune(draft.Content)) > MaxContentRunes {
		return ErrContentTooLong
	}

	return nil
}

// NormalizeAtUserIDs 规范化 @ 名单，剔除 0 与自己并去重。
//
// 返回候选集还需经应用层查询端口校验关注关系，仅互相关注的用户允许被 @。
func (draft Draft) NormalizeAtUserIDs() []uint {
	if draft.UserID == 0 || len(draft.AtUserIDs) == 0 {
		return nil
	}

	unique := make(map[uint]struct{}, len(draft.AtUserIDs))
	ids := make([]uint, 0, len(draft.AtUserIDs))

	for _, id := range draft.AtUserIDs {
		if id == 0 || id == draft.UserID {
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

// HasParent 报告草稿是否为回复（父评论 ID 有效）。
func (draft Draft) HasParent() bool {
	return draft.ParentID != nil && *draft.ParentID > 0
}

// RootForReply 依据父评论推导根评论 ID。
//
// 规则：父评论本身是根评论（RootID 为空）时取父评论 ID，
// 否则沿用父评论的 RootID，使层级恒定为两级。
func (parent ParentBrief) RootForReply(parentID uint) *uint {
	if parent.RootID != nil {
		return parent.RootID
	}

	id := parentID

	return &id
}

// DeleteDecision 删除评论的授权判定结果。
type DeleteDecision int

// WorkAuthorship 是跨上下文查询取得的作品归属事实。
type WorkAuthorship struct{ AuthorID uint }

func (w WorkAuthorship) OwnedBy(operatorID uint) bool { return w.AuthorID == operatorID }

func (c *Comment) EnsureDeletionAuthorized(operatorID uint, isWorkAuthor bool) error {
	switch c.DeletionDecision(operatorID) {
	case DeleteAllowed:
		return nil
	case DeleteNeedsVideoAuthorCheck, DeleteNeedsDynamicAuthorCheck:
		if isWorkAuthor {
			return nil
		}
	}
	return ErrDeleteForbidden
}

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

// DeletionDecision 判定删除评论所需的下一步校验。
//
// 规则：评论作者直接放行；否则按评论所属对象交由视频作者 / 动态作者校验；
// 两者皆无则拒绝。
func (c *Comment) DeletionDecision(operatorID uint) DeleteDecision {
	if c == nil {
		return DeleteDenied
	}

	if c.userID == operatorID {
		return DeleteAllowed
	}

	switch {
	case c.videoID > 0:
		return DeleteNeedsVideoAuthorCheck
	case c.dynamicID > 0:
		return DeleteNeedsDynamicAuthorCheck
	default:
		return DeleteDenied
	}
}

// EnsurePinnable 校验置顶目标：仅一级评论可置顶，且必须关联视频或动态。
func (c *Comment) EnsurePinnable() error {
	if c == nil {
		return ErrCommentNotFound
	}

	if !c.IsRoot() {
		return ErrPinRootOnly
	}

	if c.videoID == 0 && c.dynamicID == 0 {
		return ErrPinTargetMissing
	}

	return nil
}
