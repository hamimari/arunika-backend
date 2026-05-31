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

func (h *ArHandler) GetAll(c *gin.Context) {
	categoryID := c.Query("category_id")
	subCategoryID := c.Query("sub_category_id")
	cards, err := h.service.GetAll(categoryID, subCategoryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cards})
}

func (h *ArHandler) GetCategories(c *gin.Context) {
	cats, err := h.service.GetAllCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cats})
}
