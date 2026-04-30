package handlers

import (
	"arunika_backend/models"
	"arunika_backend/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type BannerHandler struct {
	svc *services.BannerService
}

func NewBannerHandler(svc *services.BannerService) *BannerHandler {
	return &BannerHandler{svc: svc}
}

// GET /banners  — mobile app home, returns active+visible banners only
func (h *BannerHandler) GetActiveBanners(c *gin.Context) {
	banners, err := h.svc.GetActiveBanners()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": banners})
}

// --- Admin handlers ---

// GET /admin/content/banners
func (h *BannerHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	items, total, err := h.svc.List(c.Query("search"), page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total, "page": page, "per_page": perPage})
}

// GET /admin/content/banners/:id
func (h *BannerHandler) Get(c *gin.Context) {
	item, err := h.svc.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

// POST /admin/content/banners
func (h *BannerHandler) Create(c *gin.Context) {
	var input models.Banner
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.Create(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

// PUT /admin/content/banners/:id
func (h *BannerHandler) Update(c *gin.Context) {
	var input models.Banner
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.Update(c.Param("id"), input)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

// DELETE /admin/content/banners/:id
func (h *BannerHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// PATCH /admin/content/banners/:id/visibility
func (h *BannerHandler) ToggleVisibility(c *gin.Context) {
	var body struct {
		Hidden bool `json:"hidden"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ToggleVisibility(c.Param("id"), body.Hidden); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"hidden": body.Hidden})
}

// PATCH /admin/content/banners/:id/active
func (h *BannerHandler) ToggleActive(c *gin.Context) {
	var body struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ToggleActive(c.Param("id"), body.IsActive); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"is_active": body.IsActive})
}
