package handlers

import (
	"arunika_backend/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AdminPaymentHandler struct {
	svc *services.AdminPaymentService
}

func NewAdminPaymentHandler(svc *services.AdminPaymentService) *AdminPaymentHandler {
	return &AdminPaymentHandler{svc: svc}
}

// GET /admin/payments?status=settlement&search=sub-123&page=1&per_page=20
func (h *AdminPaymentHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	items, total, err := h.svc.List(c.Query("status"), c.Query("search"), page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": total, "page": page, "per_page": perPage})
}

// GET /admin/payments/:id
func (h *AdminPaymentHandler) Get(c *gin.Context) {
	item, err := h.svc.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}
