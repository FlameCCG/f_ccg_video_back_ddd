// Package utils provides package functionality.
package utils

import (
	"context"
	"errors"
	"net"
	"regexp"
	"strings"
)

// Unique 切片去重升级版 泛型参数
// 利用 map 的 key 不能重复的特性 + append 函数 一次 for 循环搞定.
func Unique[T comparable](ss []T) []T {
	size := len(ss)
	if size == 0 {
		return []T{}
	}

	newSlices := make([]T, 0, size) // 这里新建一个切片, 预估大小为原切片大小, 避免频繁扩容

	m1 := make(map[T]bool)
	for _, v := range ss {
		if _, ok := m1[v]; !ok { // 如果数据不在 map 中, 放入
			m1[v] = true                     // 保存到 map 中, 用于下次判断
			newSlices = append(newSlices, v) // 将数据放入新的切片中
		}
	}

	return newSlices
}

// 将正则表达式编译一次，作为包内变量，以提高性能
// 使用了兼容性更好的正则表达式.
var emailRegex = regexp.MustCompile(`^(?i:[a-z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]+(?:\.[a-z0-9!#$%&'*+/=?^_` + "`" + `{|}~-]+)*@(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)$`)

// ValidateEmail 验证邮箱地址的格式和域名的有效性
// 如果格式正确且域名可解析到MX记录，返回 (true, "")
// 否则，返回 (false, "具体的错误信息").
func ValidateEmail(email string) (bool, string) {
	// --- 步骤 1: 格式验证 ---
	if !emailRegex.MatchString(email) {
		return false, "邮箱格式不正确"
	}

	// --- 步骤 2: DNS解析验证 ---
	// 提取域名部分
	parts := strings.Split(email, "@")
	// 理论上正则已经保证了格式，但这里做一个安全检查
	if len(parts) != 2 {
		return false, "邮箱格式不正确"
	}

	domain := parts[1]

	// 查询域名的MX(Mail Exchange)记录
	mxRecords, err := net.DefaultResolver.LookupMX(context.Background(), domain)
	if err != nil {
		// 如果错误是 "no such host"，说明域名不存在
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return false, "邮箱域名不存在"
		}
		// 其他网络错误（如超时）
		return false, "无法验证邮箱域名: " + err.Error()
	}

	// 如果找到了域名，但它没有任何MX记录，说明它无法接收邮件
	if len(mxRecords) == 0 {
		return false, "邮箱域名未配置邮件服务（缺少MX记录）"
	}

	// --- 所有检查通过 ---
	return true, ""
}
