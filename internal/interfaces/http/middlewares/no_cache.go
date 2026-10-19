package middlewares

import "github.com/gin-gonic/gin"

// NoCache prevents status probes (including error responses) from being cached by proxies.
func NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Next()
	}
}
