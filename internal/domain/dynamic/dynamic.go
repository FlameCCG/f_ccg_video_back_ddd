// Package dynamic 是动态上下文的领域层：动态聚合、置顶互斥规则、
// 「最新作品/已读」判定规则与仓储接口（纯 Go，零框架依赖）。
package dynamic

import (
	"strings"
	"time"
)

// MaxContentRunes 动态正文允许的最大字符数（按 rune 计，与原实现一致）。
const MaxContentRunes = 2000

// Dynamic 动态聚合根。
//
// json tag 与原 models/mysqlEntity.Dynamic 对外字段完全一致：
// 动态创建接口直接序列化该实体派生的读模型，响应 JSON 因此保持 1:1。
type Dynamic struct {
	ID        uint       `json:"id"`
	CreatedAt time.Time  `json:"-"`
	UpdatedAt time.Time  `json:"-"`
	UserID    uint       `json:"userId"`
	Content   string     `json:"content"`
	ImageURL  string     `json:"imageUrl"`
	LikeCount int64      `json:"likeCount"`
	IsPinned  bool       `json:"isPinned"`
	PinnedAt  *time.Time `json:"pinnedAt,omitempty"`
}

// NewDynamic 校验并构造一条待发布动态。
//
// 规则与原 DynamicService.Create 一致：未登录直接拒绝；
// 正文裁剪首尾空白后不得为空、不得超过 MaxContentRunes；图片 URL 同样裁剪空白。
func NewDynamic(userID uint, content, imageURL string) (*Dynamic, error) {
	if userID == 0 {
		return nil, ErrUserNotLogin
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return nil, ErrContentEmpty
	}

	if len([]rune(content)) > MaxContentRunes {
		return nil, ErrContentTooLong
	}

	return &Dynamic{
		UserID:   userID,
		Content:  content,
		ImageURL: strings.TrimSpace(imageURL),
	}, nil
}

// EnsureDeletableBy 校验删除权限：只有作者本人可删除自己的动态。
func (d *Dynamic) EnsureDeletableBy(userID uint) error {
	if d == nil {
		return ErrDynamicNotFound
	}

	if d.UserID != userID {
		return ErrDeleteForbidden
	}

	return nil
}

// EnsurePinnableBy 校验置顶权限：只有作者本人可置顶/取消置顶自己的动态。
func (d *Dynamic) EnsurePinnableBy(userID uint) error {
	if d == nil {
		return ErrDynamicNotFound
	}

	if d.UserID != userID {
		return ErrPinForbidden
	}

	return nil
}
