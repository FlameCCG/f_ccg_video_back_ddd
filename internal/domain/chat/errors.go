// Package chat 表达私信会话、消息与发送约束。
//
// conversation.go 定义参与者与固定会话标识，message.go/send_limit.go/paging.go 处理消息合法性、发送限制和游标。
// repository.go 声明消息持久化所需能力；Mongo BSON 与索引留在 infrastructure/persistence/chat。
//
// 本包保持纯 Go 规则与端口，不依赖 HTTP、数据库、缓存、消息框架或配置加载器。
// 领域函数尽量显式接收事实与时间，让规则可以脱离运行环境阅读和验证。
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
