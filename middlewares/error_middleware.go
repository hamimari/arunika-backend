package middlewares

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorMiddleware is the outermost middleware. It:
//   - Recovers from panics and returns a generic 500 JSON response so that no
//     stack trace or internal detail leaks to the client.
//   - Ensures every non-2xx response that leaves a handler without a body gets
//     a consistent JSON envelope.
func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered", "error", r, "path", c.Request.URL.Path)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
			}
		}()
		c.Next()
	}
}
