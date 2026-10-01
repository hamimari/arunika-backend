package handlers

import (
	"arunika_backend/services"
	"github.com/gin-gonic/gin"
	"net/http"
)

type AnimalHandler struct {
	service *services.AnimalService
}

func NewAnimalHandler(s *services.AnimalService) *AnimalHandler {
	return &AnimalHandler{service: s}
}

// GetAnimals handles GET /animals
// Optional query param: category (ternak|hutan|laut)
func (h *AnimalHandler) GetAnimals(c *gin.Context) {
	category := c.Query("category")
	animals, err := h.service.GetAnimals(category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve animals"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": animals})
}
