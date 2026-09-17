package handlers

import (
	"arunika_backend/models"
	"arunika_backend/services"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderHandler struct {
	service        *services.OrderService
	paymentService *services.PaymentService
}

func NewOrderHandler(s *services.OrderService, ps *services.PaymentService) *OrderHandler {
	return &OrderHandler{service: s, paymentService: ps}
}

// GetByID handles GET /orders/:id (owner-only; 404 for non-owners and unknown IDs)
func (h *OrderHandler) GetByID(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID, err := uuid.Parse(userIDVal.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	order, err := h.service.GetOwnedOrder(orderID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, services.ErrOrderForbidden) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve order"})
		return
	}

	// Still-PENDING orders get an active Midtrans status check so a
	// delayed/dropped webhook doesn't strand the client's poll — see
	// PaymentService.SyncOrderStatus.
	order = h.paymentService.SyncOrderStatus(order)
	c.JSON(http.StatusOK, gin.H{"data": order})
}

// Bounds on the Midtrans status checks GET /orders makes for still-PENDING
// rows, so a history page full of abandoned checkouts can't turn one list
// request into dozens of slow upstream calls. Snap links expire after 24h,
// so older PENDING orders have nothing left to reconcile.
const (
	orderListMaxSyncs   = 5
	orderListSyncMaxAge = 48 * time.Hour
)

// List handles GET /orders?page=1&per_page=20 — the authenticated user's own
// payment history, every status, newest first.
func (h *OrderHandler) List(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 50 {
		perPage = 20
	}

	orders, total, err := h.service.ListOrdersForUser(userID, page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve orders"})
		return
	}

	syncs := 0
	for i := range orders {
		if syncs >= orderListMaxSyncs {
			break
		}
		if orders[i].Status != models.OrderStatusPending || time.Since(orders[i].CreatedAt) > orderListSyncMaxAge {
			continue
		}
		syncs++
		orders[i] = *h.paymentService.SyncOrderStatus(&orders[i])
	}

	views, err := h.service.EnrichForUser(orders)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve orders"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": views, "total": total, "page": page, "per_page": perPage})
}
