// Package social 表达关注关系与关注/粉丝列表隐私规则。
//
// focus.go/repository.go 描述关注写侧，privacy.go 在调用方取得的身份与设置上判断可见性。
// 查询事实与制定规则分开：本包既不查用户表，也不负责粉丝指标或搜索索引同步。
//
// 本包保持纯 Go 规则与端口，不依赖 HTTP、数据库、缓存、消息框架或配置加载器。
// 领域函数尽量显式接收事实与时间，让规则可以脱离运行环境阅读和验证。
package social

import "time"

// Focus 关注关系聚合根：UserID 关注了 FocusUserID。
type Focus struct {
	ID          uint
	CreatedAt   time.Time
	UpdatedAt   time.Time
	UserID      uint
	FocusUserID uint
}

// NewFocus 校验并构造一条关注关系。
//
// 规则与原 api/social_api/focus_user.go 一致：不允许关注自己。
func NewFocus(userID, focusUserID uint) (*Focus, error) {
	if userID == focusUserID {
		return nil, ErrCannotFocusSelf
	}

	return &Focus{UserID: userID, FocusUserID: focusUserID}, nil
}

// RelationCounts 用户的关注/粉丝/动态计数。
type RelationCounts struct {
	FocusCount   int64
	FansCount    int64
	DynamicCount int64
}

// Relation 两个用户之间的关注关系视角（以「我」为主体）。
type Relation struct {
	// IsFocus 我是否关注了他。
	IsFocus bool
	// IsFans 他是否关注了我。
	IsFans bool
}

// IsMutual 是否互相关注。
func (r Relation) IsMutual() bool {
	return r.IsFocus && r.IsFans
}
