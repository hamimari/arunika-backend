package handlers

import (
	"arunika_backend/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
)

type CountingHandler struct {
	service *services.CountingService
}

func NewCountingHandler(s *services.CountingService) *CountingHandler {
	return &CountingHandler{service: s}
}

// GetQuestions handles GET /counting/questions
// Query params: level (string, optional) — easy|medium|hard
func (h *CountingHandler) GetQuestions(c *gin.Context) {
	level := c.Query("level")
	questions, err := h.service.GetQuestions(level)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve counting questions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": questions})
}

// SaveProgress handles POST /counting/progress
func (h *CountingHandler) SaveProgress(c *gin.Context) {
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

	var req services.SaveCountingProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	progress, err := h.service.SaveProgress(userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save counting progress"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": progress})
}
