package middlewares

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const loggerContextKey = "f_ccg_video_back_ddd/interfaces/http/logger"

func loggerOrNop(logger *zap.Logger) *zap.Logger {
	if logger == nil {
		return zap.NewNop()
	}

	return logger
}

// InjectLogger makes the explicitly injected logger available to stateless,
// generic binding middleware without introducing a package-level logger.
func (m *Middlewares) InjectLogger() gin.HandlerFunc {
	logger := m.log()

	return func(c *gin.Context) {
		c.Set(loggerContextKey, logger)
		c.Next()
	}
}

func loggerFromContext(c *gin.Context) *zap.Logger {
	if c == nil {
		return zap.NewNop()
	}

	value, ok := c.Get(loggerContextKey)
	if !ok {
		return zap.NewNop()
	}

	logger, ok := value.(*zap.Logger)
	if !ok {
		return zap.NewNop()
	}

	return loggerOrNop(logger)
}
