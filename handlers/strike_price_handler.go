package handlers

import (
	"arunika_backend/services"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StrikePriceHandler struct {
	svc *services.StrikePriceService
}

func NewStrikePriceHandler(svc *services.StrikePriceService) *StrikePriceHandler {
	return &StrikePriceHandler{svc: svc}
}

// GET /admin/strike-price-rules — the three global rules with their status.
func (h *StrikePriceHandler) AdminList(c *gin.Context) {
	rules, err := h.svc.ListRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rules})
}

// PUT /admin/strike-price-rules/:scope
// Body: {"mode": "PERCENT", "value": 20, "starts_at": "...", "ends_at": "..."}
func (h *StrikePriceHandler) AdminUpdate(c *gin.Context) {
	var body services.UpdateRuleInput
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule, err := h.svc.UpdateRule(c.Param("scope"), body)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "strike price scope not found"})
		case services.IsValidationError(err):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rule})
}
