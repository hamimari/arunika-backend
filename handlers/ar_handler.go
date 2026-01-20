package handlers

import (
	"arunika_backend/services"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type ArHandler struct {
	service *services.ArService
}

func NewArHandler(s *services.ArService) *ArHandler {
	return &ArHandler{service: s}
}

func (h *ArHandler) FindById(c *gin.Context) {
	id := c.Param("id")
	content, err := h.service.GetByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if content == nil || (content.ExpiresAt != nil && content.ExpiresAt.Before(time.Now())) {
		c.JSON(http.StatusNotFound, gin.H{"error": "content not found"})
		return
	}
	c.JSON(http.StatusOK, content)
}
