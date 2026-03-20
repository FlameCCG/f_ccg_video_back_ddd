package entity

import (
	commenterrors "f_ccg_video_back_ddd/internal/domain/comment/domainerr"
	commentvo "f_ccg_video_back_ddd/internal/domain/comment/valueobject"
	"f_ccg_video_back_ddd/internal/domain/shared/enum"
)

// Normalize 规范化发布草稿，按登录态、评论对象、内容、长度的顺序校验，并推导评论类型；同一非法请求按此顺序返回错误。
func (draft Draft) Normalize() (Draft, enum.CommentType, error) {
	if draft.UserID == 0 {
		return draft, "", commenterrors.ErrUserNotLogin
	}

	commentType, err := commentvo.ResolveTargetType(draft.VideoID, draft.DynamicID)
	if err != nil {
		return draft, "", err
	}

	draft.Content = commentvo.NormalizeContent(draft.Content)
	if err := commentvo.ValidateContent(draft.Content, len(draft.Pictures)); err != nil {
		return draft, "", err
	}

	return draft, commentType, nil
}

// HasParent 报告草稿是否为回复（父评论 ID 有效）。
func HasParent(parentID *uint) bool {
	return parentID != nil && *parentID > 0
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

// EnsurePinnable 校验置顶目标：仅一级评论可置顶，且必须关联视频或动态。
func (c *Comment) EnsurePinnable() error {
	if c == nil {
		return commenterrors.ErrCommentNotFound
	}

	if !c.IsRoot() {
		return commenterrors.ErrPinRootOnly
	}

	if c.VideoID == 0 && c.DynamicID == 0 {
		return commenterrors.ErrPinTargetMissing
	}

	return nil
}
