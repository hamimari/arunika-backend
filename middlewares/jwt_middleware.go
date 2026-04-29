package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// JWTAuthMiddleware validates a Bearer token, checks the Redis blacklist and
// sets user information on the gin context for downstream handlers.
func JWTAuthMiddleware(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Read the secret at request time so it is always current and never
		// evaluated before godotenv.Load() has been called.
		secretKey := os.Getenv("JWT_SECRET")
		if secretKey == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfiguration"})
			c.Abort()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			c.Abort()
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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			c.Abort()
			return
		}

		// Expiry is already validated by jwt.Parse, but we double-check here
		// to guard against tokens without an exp claim.
		expFloat, ok := claims["exp"].(float64)
		if !ok || float64(time.Now().Unix()) > expFloat {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
			return
		}

		jti, ok := claims["jti"].(string)
		if !ok || jti == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		// Check Redis blacklist (populated on logout)
		ctx := context.Background()
		revoked, _ := rdb.Get(ctx, "blacklist:"+jti).Result()
		if revoked == "revoked" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token revoked"})
			return
		}

		c.Set("userID", claims["sub"])
		c.Set("email", claims["email"])
		c.Set("refresh_token", claims["refresh_token"])
		c.Set("jti", jti)
		c.Set("exp", time.Unix(int64(expFloat), 0))

		c.Next()
	}
}
