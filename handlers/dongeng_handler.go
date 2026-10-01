package handlers

import (
	"arunika_backend/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

	categoryID := c.Query("dongeng_category_id")
	subCategoryID := c.Query("dongeng_sub_category_id")

	result, err := h.service.GetFairyTales(search, page, perPage, optionalUserID(c), categoryID, subCategoryID)
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

// GetCategories handles GET /dongeng-categories.
func (h *DongengHandler) GetCategories(c *gin.Context) {
	cats, err := h.service.GetCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cats})
}

// GetFairyTaleByID handles GET /fairy-tales/:id
// Returns a single dongeng with its pages ordered by page_number.
func (h *DongengHandler) GetFairyTaleByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	dongeng, err := h.service.GetFairyTaleByID(id, optionalUserID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "fairy tale not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": dongeng})
}

// RecordPlay handles POST /fairy-tales/:id/play. Guests (no authenticated
// user) have nothing to attribute a play to — treated as a no-op success
// rather than a 401, since this is a best-effort analytics call that must
// never block a guest from watching free content.
func (h *DongengHandler) RecordPlay(c *gin.Context) {
	userID := optionalUserID(c)
	if userID == nil {
		c.JSON(http.StatusOK, gin.H{"status": "skipped"})
		return
	}
	dongengID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dongeng id"})
		return
	}
	if err := h.service.RecordPlay(*userID, dongengID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record play"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type UpdateProgressRequest struct {
	ProgressSeconds int `json:"progress_seconds" binding:"required"`
}

// UpdateProgressHandler handles PUT /fairy-tales/:id/play
func (h *DongengHandler) UpdateProgressHandler(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	dongengID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dongeng id"})
		return
	}
	var req UpdateProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	if err := h.service.UpdateProgress(userID, dongengID, req.ProgressSeconds); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update progress"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetHistory handles GET /fairy-tales/history
func (h *DongengHandler) GetHistory(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	items, err := h.service.GetHistory(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve history"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}
