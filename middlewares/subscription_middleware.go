package middlewares

import (
	"arunika_backend/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"net/http"
	"time"
)

// SubscriptionMiddleware returns a Gin middleware that rejects free-tier users with HTTP 403.
func SubscriptionMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, exists := c.Get("userID")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		userID, err := uuid.Parse(userIDVal.(string))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}

		var sub models.UserSubscription
		if err := db.Where("user_id = ?", userID).First(&sub).Error; err != nil {
			// No subscription row → treat as free.
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "premium subscription required"})
			return
		}

		isPremium := sub.Status == "premium" &&
			(sub.ExpiresAt == nil || sub.ExpiresAt.After(time.Now()))
		if !isPremium {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "premium subscription required"})
			return
		}

		c.Next()
	}
}
