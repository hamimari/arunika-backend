package handlers

import (
	"arunika_backend/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AdminOrderHandler struct {
	svc        *services.OrderService
	paymentSvc *services.PaymentService
}

func NewAdminOrderHandler(svc *services.OrderService, paymentSvc *services.PaymentService) *AdminOrderHandler {
	return &AdminOrderHandler{svc: svc, paymentSvc: paymentSvc}
}

// GET /admin/orders?status=PAID&search=&page=1&per_page=20
func (h *AdminOrderHandler) List(c *gin.Context) {
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

// POST /admin/orders/:id/sync — force a re-check of the order's status
// against Midtrans instead of waiting for a webhook delivery.
func (h *AdminOrderHandler) Sync(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	order, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	order = h.paymentSvc.SyncOrderStatus(order)

	views, err := h.svc.EnrichOne(*order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": views})
}
