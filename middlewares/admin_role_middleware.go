package middlewares

import (
	"net/http"

	"arunika_backend/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RequireAdminRole allows the request only when the admin's current role is
// one of roles. Must run after AdminAuthMiddleware. The role is read from the
// database on every request rather than from the token, so a demotion takes
// effect immediately instead of when the access token expires.
func RequireAdminRole(db *gorm.DB, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		adminID, _ := c.Get("adminID")
		var role string
		err := db.Model(&models.AdminUser{}).
			Where("id = ? AND is_deleted = false", adminID).
			Pluck("role", &role).Error
		if err != nil || role == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required", "code": "ROLE_REQUIRED"})
			return
		}
		for _, r := range roles {
			if role == r {
				c.Set("adminRole", role)
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "this action needs the publisher role", "code": "ROLE_REQUIRED"})
	}
}
