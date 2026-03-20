// Package utils 提供接口入口使用的分页参数辅助。
// 业务规则应放入所属 domain/application，避免工具包承载领域职责。
package utils

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetPageInfo 从请求中获取分页参数.
func GetPageInfo(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	if page < 1 {
		page = 1
	}

	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	return page, pageSize
}
