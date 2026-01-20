package handlers

import (
	"arunika_backend/services"
	"github.com/gin-gonic/gin"
	"net/http"
)

type DongengHandler struct {
	service *services.DongengService
}

func NewDongengHandler(s *services.DongengService) *DongengHandler {
	return &DongengHandler{service: s}
}

func (h *DongengHandler) GetFairyTales(c *gin.Context) {
	dongeng, err := h.service.GetFairyTales()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": dongeng})
}
