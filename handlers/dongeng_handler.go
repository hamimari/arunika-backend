package handlers

import (
	"arunika_backend/services"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type DongengHandler struct {
	service *services.DongengService
}

func NewDongengHandler(s *services.DongengService) *DongengHandler {
	return &DongengHandler{service: s}
}

// GetFairyTales handles GET /fairy-tales
// Query params: search (string), page (int, default 1), per_page (int, default 10, max 100)
func (h *DongengHandler) GetFairyTales(c *gin.Context) {
	search := c.Query("search")

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	perPage, err := strconv.Atoi(c.DefaultQuery("per_page", "10"))
	if err != nil || perPage < 1 || perPage > 100 {
		perPage = 10
	}

	result, err := h.service.GetFairyTales(search, page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve fairy tales"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     result.Items,
		"total":    result.Total,
		"page":     result.Page,
		"per_page": result.PerPage,
	})
}

// GetFairyTaleByID handles GET /fairy-tales/:id
// Returns a single dongeng with its pages ordered by page_number.
func (h *DongengHandler) GetFairyTaleByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	dongeng, err := h.service.GetFairyTaleByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "fairy tale not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": dongeng})
}
