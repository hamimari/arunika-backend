package handlers

import (
	"arunika_backend/services"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AdminCampaignHandler struct {
	svc *services.AdminCampaignService
}

func NewAdminCampaignHandler(svc *services.AdminCampaignService) *AdminCampaignHandler {
	return &AdminCampaignHandler{svc: svc}
}

// POST /admin/campaigns — records the campaign and starts delivery in the
// background; responds 202 with the SENDING campaign row.
func (h *AdminCampaignHandler) Dispatch(c *gin.Context) {
	var req services.CampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var adminID *uuid.UUID
	if raw, ok := c.Get("adminID"); ok {
		if str, ok := raw.(string); ok {
			if id, err := uuid.Parse(str); err == nil {
				adminID = &id
			}
		}
	}

	campaign, err := h.svc.Dispatch(req, adminID)
	if err != nil {
		var validationErr *services.CampaignValidationError
		if errors.As(err, &validationErr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": validationErr.Error()})
			return
		}
		if errors.Is(err, services.ErrPushUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": campaign})
}

// GET /admin/campaigns?page=1&per_page=20
func (h *AdminCampaignHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	items, total, err := h.svc.List(page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total, "page": page, "per_page": perPage})
}
