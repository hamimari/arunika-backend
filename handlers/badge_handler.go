package handlers

import (
	"arunika_backend/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
)

type BadgeHandler struct {
	service *services.BadgeService
}

func NewBadgeHandler(s *services.BadgeService) *BadgeHandler {
	return &BadgeHandler{service: s}
}

// GetBadges handles GET /badges
// Returns all badge definitions with earned status and progress for the authenticated user.
func (h *BadgeHandler) GetBadges(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID, err := uuid.Parse(userIDVal.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	badges, err := h.service.GetUserBadges(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve badges"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": badges})
}
