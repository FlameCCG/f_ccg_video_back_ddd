// Package casbins 生成 Casbin 用户主体标识。
//
// GetUserSub 将用户 ID 转成 user:<id>，用于区分用户主体与角色名称。
// 授权策略读写双方必须使用同一种前缀，改变格式会影响已存在的策略。
package casbins

import "fmt"

// GetUserSub 将用户标识编码成 user:<id> 的 Casbin subject，保持策略存取双方使用一致前缀。
func GetUserSub(uid any) string {
	return fmt.Sprintf("user:%v", uid)
}
