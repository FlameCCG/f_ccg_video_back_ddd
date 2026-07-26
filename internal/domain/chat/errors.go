// Package chat 是私信上下文的领域层：会话/消息实体、参与者规则、
// 发送频率限制规则、未读推进规则与分页游标规则（纯 Go，零框架依赖）。
package chat

import "errors"

// 领域错误。错误文案属于外部契约（会原样返回给前端），不得修改。
var (
	// ErrStoreUnavailable 私信存储（Mongo）未初始化。
	//
	// 文案沿用原 chat_service.getChatDB 的返回值，属于外部可观测契约。
	ErrStoreUnavailable = errors.New("mongo 未初始化")
	// ErrNotLoggedIn 当前用户未登录（userID 为 0）。
	ErrNotLoggedIn = errors.New("用户未登录")
	// ErrSelfMessage 不允许给自己发消息（发消息路径）。
	ErrSelfMessage = errors.New("不能给自己发消息")
	// ErrSelfConversation 不允许给自己创建私信会话（建会话路径）。
	ErrSelfConversation = errors.New("不能给自己发私信")
	// ErrEmptyPeerID 对端用户 ID 为空。
	ErrEmptyPeerID = errors.New("peerId 不能为空")
	// ErrInvalidPeerID 对端用户 ID 非法（与自己相同）。
	ErrInvalidPeerID = errors.New("peerId 无效")
	// ErrEmptyText 文本消息内容为空。
	ErrEmptyText = errors.New("消息内容不能为空")
	// ErrTextTooLong 文本消息超过 1000 字。
	ErrTextTooLong = errors.New("文字内容最多1000字")
	// ErrEmptyEmoji emoji 消息内容为空。
	ErrEmptyEmoji = errors.New("emoji 不能为空")
	// ErrEmptyMediaURL 图片/表情包消息缺少地址。
	ErrEmptyMediaURL = errors.New("图片地址不能为空")
	// ErrUnsupportedMessageType 不支持的消息类型。
	ErrUnsupportedMessageType = errors.New("不支持的消息类型")
	// ErrInvalidCursor before 游标不是合法的 24 位十六进制消息 ID。
	ErrInvalidCursor = errors.New("before 参数无效")
	// ErrConversationNotFound 会话不存在（upsert 后仍未返回文档）。
	ErrConversationNotFound = errors.New("会话不存在")
	// ErrDailySendLimit 对端未关注自己时的单日发送限制。
	ErrDailySendLimit = errors.New("对方未关注你，今日仅可发送1条消息，等待对方回复后可继续聊天")
)
