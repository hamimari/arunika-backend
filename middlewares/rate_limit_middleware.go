package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimitMiddleware applies a simple fixed-window request limit per client
// IP, keyed by keyPrefix, backed by Redis (INCR + EXPIRE). Intended for
// unauthenticated, abuse-prone endpoints (e.g. forgot/reset password) that
// have no other rate limiting today.
//
// Redis errors fail open — a Redis outage must not take the protected
// endpoint down, only the rate limit guarding it.
func RateLimitMiddleware(rdb *redis.Client, keyPrefix string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil {
			c.Next()
			return
		}

		ctx := context.Background()
		key := fmt.Sprintf("ratelimit:%s:%s", keyPrefix, c.ClientIP())

		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if count == 1 {
			rdb.Expire(ctx, key, window)
		}
		if count > int64(limit) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests. Please try again later.",
			})
			return
		}

		c.Next()
	}
}
