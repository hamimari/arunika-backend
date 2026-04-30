package handlers

import (
	"arunika_backend/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
)

type GrowthHandler struct {
	service *services.GrowthService
}

func NewGrowthHandler(s *services.GrowthService) *GrowthHandler {
	return &GrowthHandler{service: s}
}

// SaveRecord handles POST /growth
func (h *GrowthHandler) SaveRecord(c *gin.Context) {
	var req services.SaveGrowthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	record, err := h.service.Save(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save growth record"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": record})
}

// UpdateRecord handles PUT /growth/:id
func (h *GrowthHandler) UpdateRecord(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req services.UpdateGrowthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	record, err := h.service.Update(id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update growth record"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": record})
}

func (h *GrowthHandler) GetHistory(c *gin.Context) {
	childIDStr := c.Query("child_id")
	if childIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "child_id is required"})
		return
	}
	childID, err := uuid.Parse(childIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid child_id"})
		return
	}

	records, err := h.service.GetHistory(childID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve growth records"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": records})
}
