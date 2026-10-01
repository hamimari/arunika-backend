package middlewares

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// OptionalAuthMiddleware behaves like JWTAuthMiddleware when a valid Bearer
// token is present (setting the same context keys), but never aborts the
// request when a token is missing or invalid — downstream handlers just see
// an unauthenticated request. Used for endpoints that serve both public
// (free-only) and personalized (entitlement-aware) responses.
func OptionalAuthMiddleware(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		secretKey := os.Getenv("JWT_SECRET")
		if secretKey == "" {
			c.Next()
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(secretKey), nil
		})
		if err != nil || !token.Valid {
			c.Next()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.Next()
			return
		}

		expFloat, ok := claims["exp"].(float64)
		if !ok || float64(time.Now().Unix()) > expFloat {
			c.Next()
			return
		}

		jti, ok := claims["jti"].(string)
		if !ok || jti == "" {
			c.Next()
			return
		}

		revoked, _ := rdb.Get(context.Background(), "blacklist:"+jti).Result()
		if revoked == "revoked" {
			c.Next()
			return
		}

		c.Set("userID", claims["sub"])
		c.Set("email", claims["email"])
		c.Set("jti", jti)
		c.Set("exp", time.Unix(int64(expFloat), 0))
		c.Next()
	}
}
