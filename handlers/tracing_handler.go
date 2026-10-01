package handlers

import (
	"arunika_backend/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
)

type TracingHandler struct {
	service *services.TracingService
}

func NewTracingHandler(s *services.TracingService) *TracingHandler {
	return &TracingHandler{service: s}
}

// GetItems handles GET /tracing/items
// Query params: type (string, optional) — filter by alphabet|number|shape
func (h *TracingHandler) GetItems(c *gin.Context) {
	itemType := c.Query("type")
	items, err := h.service.GetItems(itemType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve tracing items"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// SaveProgress handles POST /tracing/progress
func (h *TracingHandler) SaveProgress(c *gin.Context) {
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

	var req services.SaveTracingProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	progress, err := h.service.SaveProgress(userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save tracing progress"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": progress})
}
