package middlewares

import (
	"errors"
	"net"               // 网络错误判断（如断开的连接）
	"net/http"          // HTTP 响应状态码
	"net/http/httputil" // 用于抓取请求的完整原始数据
	"os"                // 系统调用，标准输出、错误类型判断等
	"runtime/debug"     // 获取 panic 栈信息
	"strings"           // 字符串工具函数
	"time"              // 时间处理

	"github.com/gin-gonic/gin" // Gin Web 框架
	"go.uber.org/zap"          // Uber 开发的高性能结构化日志库
)

// GinLogger Gin 日志中间件，记录每个请求的基本信息（优化了颜色显示）.
//
// 注意：与原项目一致，该中间件当前未挂载到引擎上（引擎使用 gin.Default() 自带的
// Logger/Recovery）。为不丢弃任何原有能力而完整迁移，日志器改为显式传入。
func GinLogger(lg *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()             // 请求开始时间
		path := c.Request.URL.Path      // 请求路径
		query := c.Request.URL.RawQuery // 请求 Query 参数

		c.Next() // 继续处理请求（执行后续中间件与控制器）

		// 请求结束，记录日志
		cost := time.Since(start)
		status := c.Writer.Status()

		// 根据状态码选择日志级别
		logFunc := lg.Info
		if status >= http.StatusBadRequest && status < http.StatusInternalServerError {
			logFunc = lg.Warn
		} else if status >= http.StatusInternalServerError {
			logFunc = lg.Error
		}

		logFunc("HTTP Request",
			zap.Int("status", status),                                          // HTTP 状态码
			zap.String("method", c.Request.Method),                             // 请求方法
			zap.String("path", path),                                           // 路径
			zap.String("query", query),                                         // query 字符串
			zap.String("ipUtils", c.ClientIP()),                                // 客户端 IP
			zap.String("user-agent", c.Request.UserAgent()),                    // UA
			zap.String("errs", c.Errors.ByType(gin.ErrorTypePrivate).String()), // gin 绑定时产生的错误
			zap.Duration("cost", cost),                                         // 请求耗时
		)
	}
}

// GinRecovery Gin 异常捕获中间件，防止 panic 崩溃服务器（优化了错误显示）.
//
// 注意：与 GinLogger 相同，原项目未挂载该中间件；此处完整迁移并显式注入日志器。
func GinRecovery(lg *zap.Logger, stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 检查是否为断开连接类 panic（客户端取消连接）
				var brokenPipe bool

				if e, ok := err.(error); ok {
					var ne *net.OpError
					if errors.As(e, &ne) {
						var se *os.SyscallError
						if errors.As(ne.Err, &se) {
							if strings.Contains(strings.ToLower(se.Error()), "broken pipe") ||
								strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
								brokenPipe = true
							}
						}
					}
				}

				// 获取原始请求内容（不包含 body）
				httpRequest, _ := httputil.DumpRequest(c.Request, false)

				if brokenPipe {
					// 客户端断开连接，只记录简单错误
					lg.Error("Client Connection Broken",
						zap.Any("error", err),
						zap.String("path", c.Request.URL.Path),
						zap.String("method", c.Request.Method),
						zap.String("ipUtils", c.ClientIP()),
					)

					if e, ok := err.(error); ok {
						_ = c.Error(e) // 将错误加入 gin 的 context error 中
					}

					c.Abort()

					return
				}

				// 记录完整 panic 错误和请求
				if stack {
					lg.Error("Recovery from Panic",
						zap.Any("error", err),
						zap.String("path", c.Request.URL.Path),
						zap.String("method", c.Request.Method),
						zap.String("ipUtils", c.ClientIP()),
						zap.String("request", string(httpRequest)),
						zap.String("stack", string(debug.Stack())), // 打印完整堆栈
					)
				} else {
					lg.Error("Recovery from Panic",
						zap.Any("error", err),
						zap.String("path", c.Request.URL.Path),
						zap.String("method", c.Request.Method),
						zap.String("ipUtils", c.ClientIP()),
						zap.String("request", string(httpRequest)),
					)
				}

				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()

		c.Next()
	}
}
