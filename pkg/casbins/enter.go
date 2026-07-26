// Package casbins provides package functionality.
package casbins

import "fmt"

// GetUserSub 导出符号.
func GetUserSub(uid any) string {
	return fmt.Sprintf("user:%v", uid)
}
