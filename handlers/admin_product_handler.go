package handlers

import (
	"arunika_backend/services"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AdminProductHandler struct {
	svc *services.ProductService
}

func NewAdminProductHandler(svc *services.ProductService) *AdminProductHandler {
	return &AdminProductHandler{svc: svc}
}

// GET /admin/products
func (h *AdminProductHandler) List(c *gin.Context) {
	items, err := h.svc.ListEnriched()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// GET /admin/products/:id
func (h *AdminProductHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}
	item, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

// POST /admin/products
// Body: { "feature_code": "AR_CARD"|"DONGENG", "price_idr": 29000, "ar_card_id": "...", "dongeng_id": "..." }
func (h *AdminProductHandler) Create(c *gin.Context) {
	var body struct {
		FeatureCode   string  `json:"feature_code" binding:"required"`
		PriceIdr      int64   `json:"price_idr"    binding:"required"`
		ArCardID      string  `json:"ar_card_id"`
		DongengID     *string `json:"dongeng_id"`
		PlayProductID *string `json:"play_product_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := services.CreateProductInput{
		FeatureCode:   body.FeatureCode,
		PriceIdr:      body.PriceIdr,
		ArCardID:      body.ArCardID,
		PlayProductID: body.PlayProductID,
	}
	if body.DongengID != nil && *body.DongengID != "" {
		dongengID, err := uuid.Parse(*body.DongengID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dongeng_id"})
			return
		}
		input.DongengID = &dongengID
	}

	product, err := h.svc.Create(input)
	if services.IsValidationError(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": product})
}

// PUT /admin/products/:id
// Body: { "price_idr": 39000, "strike_mode": "PERCENT", "strike_value": 20,
// "strike_starts_at": "...", "strike_ends_at": "..." }. strike_mode is
// PERCENT|FIXED|NONE; a null/absent strike_mode clears the override so the
// product inherits its global rule.
func (h *AdminProductHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}
	var body struct {
		PriceIdr int64 `json:"price_idr" binding:"required"`
		// PlayProductID: absent leaves the mapping alone, null or "" clears
		// it, a string sets it.
		PlayProductID json.RawMessage `json:"play_product_id"`
		services.StrikeInput
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var play services.PlayProductUpdate
	if len(body.PlayProductID) > 0 {
		play.Set = true
		if err := json.Unmarshal(body.PlayProductID, &play.SKU); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "play_product_id must be a string or null"})
			return
		}
	}
	product, err := h.svc.Update(id, body.PriceIdr, body.StrikeInput, play)
	if services.IsValidationError(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": product})
}

// DELETE /admin/products/:id
func (h *AdminProductHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}
	if err := h.svc.Delete(id); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// PATCH /admin/products/:id/active
// Body: { "is_active": false }
func (h *AdminProductHandler) ToggleActive(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}
	var body struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ToggleActive(id, body.IsActive); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"is_active": body.IsActive})
}
