// Package emailStore 定义邮件验证码在存储边界传递的数据。
//
// EmailStoreInfo 组合邮箱、验证码和生成时间，不负责生成、发送、限流或验证。
// 实际 Redis 存取位于 infrastructure/captcha，业务流程在 application/identity。
package emailStore

import "time"

// EmailStoreInfo 是验证码存储记录：Email 为目标邮箱，Code 为待验证值，Timestamp 为生成时间。
type EmailStoreInfo struct {
	Email     string
	Code      string
	Timestamp time.Time
}
