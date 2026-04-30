package handlers

import (
	"arunika_backend/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AdminCampaignHandler struct {
	svc *services.AdminCampaignService
}

func NewAdminCampaignHandler(svc *services.AdminCampaignService) *AdminCampaignHandler {
	return &AdminCampaignHandler{svc: svc}
}

// POST /admin/campaigns
func (h *AdminCampaignHandler) Dispatch(c *gin.Context) {
	var req services.CampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.svc.Dispatch(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": result})
}
