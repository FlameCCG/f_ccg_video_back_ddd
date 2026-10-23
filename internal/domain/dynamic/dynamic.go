// Package dynamic 表达动态实体、作品活动、置顶与已读规则。
//
// dynamic.go/repository.go 构成写侧模型，activity.go/feed.go/notice.go/read.go 描述活动和阅读事实的判定。
// 跨视频与动态的置顶互斥需要应用用例取得事实并协调持久化；纯规则本身不访问视频数据库。
//
// 本包保持纯 Go 规则与端口，不依赖 HTTP、数据库、缓存、消息框架或配置加载器。
// 领域函数尽量显式接收事实与时间，让规则可以脱离运行环境阅读和验证。
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
