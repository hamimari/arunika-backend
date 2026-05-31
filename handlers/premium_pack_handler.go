package handlers

import (
	"arunika_backend/services"
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
)

type PremiumPackHandler struct {
	service *services.PremiumPackService
}

func NewPremiumPackHandler(s *services.PremiumPackService) *PremiumPackHandler {
	return &PremiumPackHandler{service: s}
}

// GetActivePacks handles GET /premium/packs (public — no auth required)
// Optional ?type=content|subscription query param.
func (h *PremiumPackHandler) GetActivePacks(c *gin.Context) {
	packType := c.Query("type")
	packs, err := h.service.GetActivePacks(packType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve packages"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": packs})
}

// AdminListPacks handles GET /admin/premium/packs (admin JWT required)
func (h *PremiumPackHandler) AdminListPacks(c *gin.Context) {
	packs, err := h.service.GetAllPacks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve packages"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": packs})
}

// AdminCreatePack handles POST /admin/premium/packs (admin JWT required)
func (h *PremiumPackHandler) AdminCreatePack(c *gin.Context) {
	var input services.CreatePremiumPackInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pack, err := h.service.CreatePack(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create package"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": pack})
}

// AdminUpdatePack handles PUT /admin/premium/packs/:id (admin JWT required)
func (h *PremiumPackHandler) AdminUpdatePack(c *gin.Context) {
	id := c.Param("id")
	var input services.UpdatePremiumPackInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pack, err := h.service.UpdatePack(id, input)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update package"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": pack})
}

// AdminDeletePack handles DELETE /admin/premium/packs/:id (admin JWT required)
func (h *PremiumPackHandler) AdminDeletePack(c *gin.Context) {
	id := c.Param("id")
	err := h.service.DeletePack(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete package"})
		return
	}
	c.Status(http.StatusNoContent)
}

// AdminToggleVisibility handles PATCH /admin/premium/packs/:id/visibility (admin JWT required)
func (h *PremiumPackHandler) AdminToggleVisibility(c *gin.Context) {
	id := c.Param("id")
	var input services.ToggleVisibilityInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pack, err := h.service.ToggleVisibility(id, input.IsActive)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update visibility"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": pack})
}
