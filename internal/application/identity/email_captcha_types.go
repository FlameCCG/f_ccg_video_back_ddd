package identity

import (
	"time"
)

// EmailCaptchaSendInput 发送邮箱验证码入参。
type EmailCaptchaSendInput struct {
	// Type 1 注册 / 2 重置密码 / 3 绑定邮箱。
	Type int
	// Email 目标邮箱。
	Email string
	// ClientIP 请求方 IP，用于限流维度。
	ClientIP string
}

// EmailCaptchaSendResult 发送邮箱验证码结果。
type EmailCaptchaSendResult struct {
	// EmailID 验证码在存储中的索引 ID，需回传给校验接口。
	EmailID string
	// Cost 从进入用例到提交异步发送的耗时。
	Cost time.Duration
}
