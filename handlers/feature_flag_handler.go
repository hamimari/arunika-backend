package handlers

import (
	"arunika_backend/services"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type FeatureFlagHandler struct {
	svc *services.FeatureFlagService
}

func NewFeatureFlagHandler(svc *services.FeatureFlagService) *FeatureFlagHandler {
	return &FeatureFlagHandler{svc: svc}
}

// GET /app/feature-flags — public; returns {"data": {"qr_scan": true, ...}}
func (h *FeatureFlagHandler) GetPublic(c *gin.Context) {
	flags, err := h.svc.EnabledMap()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve feature flags"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": flags})
}

// GET /admin/feature-flags
func (h *FeatureFlagHandler) AdminList(c *gin.Context) {
	flags, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": flags})
}

// PATCH /admin/feature-flags/:key  {"is_enabled": false}
func (h *FeatureFlagHandler) AdminToggle(c *gin.Context) {
	var body struct {
		IsEnabled *bool `json:"is_enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "is_enabled is required"})
		return
	}
	flag, err := h.svc.SetEnabled(c.Param("key"), *body.IsEnabled)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "feature flag not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": flag})
}
