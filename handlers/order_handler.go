package handlers

import (
	"arunika_backend/services"
	"errors"
	"net/http"

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
