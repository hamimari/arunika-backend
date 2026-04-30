package handlers

import (
	"arunika_backend/models"
	"arunika_backend/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AdminContentHandler struct {
	svc *services.AdminContentService
}

func NewAdminContentHandler(svc *services.AdminContentService) *AdminContentHandler {
	return &AdminContentHandler{svc: svc}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func pageParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	return page, perPage
}

func visibilityBody(c *gin.Context) (bool, error) {
	var body struct {
		Hidden bool `json:"hidden"`
	}
	err := c.ShouldBindJSON(&body)
	return body.Hidden, err
}

// ─── Fairy Tales ──────────────────────────────────────────────────────────────

func (h *AdminContentHandler) ListFairyTales(c *gin.Context) {
	page, perPage := pageParams(c)
	items, total, err := h.svc.ListFairyTales(c.Query("search"), page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total, "page": page, "per_page": perPage})
}

func (h *AdminContentHandler) GetFairyTale(c *gin.Context) {
	item, err := h.svc.GetFairyTale(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *AdminContentHandler) CreateFairyTale(c *gin.Context) {
	var input models.Dongeng
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.CreateFairyTale(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func (h *AdminContentHandler) UpdateFairyTale(c *gin.Context) {
	var input models.Dongeng
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.UpdateFairyTale(c.Param("id"), input)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *AdminContentHandler) DeleteFairyTale(c *gin.Context) {
	if err := h.svc.DeleteFairyTale(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *AdminContentHandler) ToggleFairyTaleVisibility(c *gin.Context) {
	hidden, err := visibilityBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ToggleFairyTaleVisibility(c.Param("id"), hidden); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"hidden": hidden})
}

// ─── AR Cards ────────────────────────────────────────────────────────────────

func (h *AdminContentHandler) ListArCards(c *gin.Context) {
	page, perPage := pageParams(c)
	items, total, err := h.svc.ListArCards(c.Query("search"), page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total, "page": page, "per_page": perPage})
}

func (h *AdminContentHandler) GetArCard(c *gin.Context) {
	item, err := h.svc.GetArCard(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *AdminContentHandler) CreateArCard(c *gin.Context) {
	var input models.ArCards
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.CreateArCard(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func (h *AdminContentHandler) UpdateArCard(c *gin.Context) {
	var input models.ArCards
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.UpdateArCard(c.Param("id"), input)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *AdminContentHandler) DeleteArCard(c *gin.Context) {
	if err := h.svc.DeleteArCard(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *AdminContentHandler) ToggleArCardVisibility(c *gin.Context) {
	hidden, err := visibilityBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ToggleArCardVisibility(c.Param("id"), hidden); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"hidden": hidden})
}

// ─── Tracing Items ────────────────────────────────────────────────────────────

func (h *AdminContentHandler) ListTracingItems(c *gin.Context) {
	page, perPage := pageParams(c)
	items, total, err := h.svc.ListTracingItems(c.Query("search"), page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total, "page": page, "per_page": perPage})
}

func (h *AdminContentHandler) GetTracingItem(c *gin.Context) {
	item, err := h.svc.GetTracingItem(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *AdminContentHandler) CreateTracingItem(c *gin.Context) {
	var input models.TracingItem
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.CreateTracingItem(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func (h *AdminContentHandler) UpdateTracingItem(c *gin.Context) {
	var input models.TracingItem
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.UpdateTracingItem(c.Param("id"), input)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *AdminContentHandler) DeleteTracingItem(c *gin.Context) {
	if err := h.svc.DeleteTracingItem(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *AdminContentHandler) ToggleTracingItemVisibility(c *gin.Context) {
	hidden, err := visibilityBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ToggleTracingItemVisibility(c.Param("id"), hidden); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"hidden": hidden})
}

// ─── Counting Questions ───────────────────────────────────────────────────────

func (h *AdminContentHandler) ListCountingQuestions(c *gin.Context) {
	page, perPage := pageParams(c)
	items, total, err := h.svc.ListCountingQuestions(c.Query("search"), page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total, "page": page, "per_page": perPage})
}

func (h *AdminContentHandler) GetCountingQuestion(c *gin.Context) {
	item, err := h.svc.GetCountingQuestion(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *AdminContentHandler) CreateCountingQuestion(c *gin.Context) {
	var input models.CountingQuestion
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.CreateCountingQuestion(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func (h *AdminContentHandler) UpdateCountingQuestion(c *gin.Context) {
	var input models.CountingQuestion
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.UpdateCountingQuestion(c.Param("id"), input)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *AdminContentHandler) DeleteCountingQuestion(c *gin.Context) {
	if err := h.svc.DeleteCountingQuestion(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *AdminContentHandler) ToggleCountingQuestionVisibility(c *gin.Context) {
	hidden, err := visibilityBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ToggleCountingQuestionVisibility(c.Param("id"), hidden); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"hidden": hidden})
}

// ─── Badges ───────────────────────────────────────────────────────────────────

func (h *AdminContentHandler) ListBadges(c *gin.Context) {
	page, perPage := pageParams(c)
	items, total, err := h.svc.ListBadges(c.Query("search"), page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total, "page": page, "per_page": perPage})
}

func (h *AdminContentHandler) GetBadge(c *gin.Context) {
	item, err := h.svc.GetBadge(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *AdminContentHandler) CreateBadge(c *gin.Context) {
	var input models.Badge
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.CreateBadge(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func (h *AdminContentHandler) UpdateBadge(c *gin.Context) {
	var input models.Badge
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.UpdateBadge(c.Param("id"), input)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *AdminContentHandler) DeleteBadge(c *gin.Context) {
	if err := h.svc.DeleteBadge(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *AdminContentHandler) ToggleBadgeVisibility(c *gin.Context) {
	hidden, err := visibilityBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ToggleBadgeVisibility(c.Param("id"), hidden); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"hidden": hidden})
}

// ─── Categories ───────────────────────────────────────────────────────────────

func (h *AdminContentHandler) ListCategories(c *gin.Context) {
	page, perPage := pageParams(c)
	items, total, err := h.svc.ListCategories(c.Query("search"), page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total, "page": page, "per_page": perPage})
}

func (h *AdminContentHandler) GetCategory(c *gin.Context) {
	item, err := h.svc.GetCategory(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *AdminContentHandler) CreateCategory(c *gin.Context) {
	var input models.Categories
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.CreateCategory(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func (h *AdminContentHandler) UpdateCategory(c *gin.Context) {
	var input models.Categories
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := h.svc.UpdateCategory(c.Param("id"), input)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (h *AdminContentHandler) DeleteCategory(c *gin.Context) {
	if err := h.svc.DeleteCategory(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *AdminContentHandler) ToggleCategoryVisibility(c *gin.Context) {
	hidden, err := visibilityBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ToggleCategoryVisibility(c.Param("id"), hidden); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"hidden": hidden})
}
