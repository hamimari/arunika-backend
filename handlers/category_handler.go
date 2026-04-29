package handlers

import (
	"arunika_backend/services"
	"github.com/gin-gonic/gin"
	"net/http"
)

type CategoryHandler struct {
	service *services.CategoryService
}

func NewCategoryHandler(s *services.CategoryService) *CategoryHandler {
	return &CategoryHandler{service: s}
}

func (h *CategoryHandler) GetCategories(c *gin.Context) {
	categories, err := h.service.GetCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve categories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": categories})
}
