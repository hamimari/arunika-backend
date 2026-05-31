package handlers

import (
	"arunika_backend/services"
	"github.com/gin-gonic/gin"
	"net/http"
)

type PaymentHandler struct {
	service *services.PaymentService
}

func NewPaymentHandler(s *services.PaymentService) *PaymentHandler {
	return &PaymentHandler{service: s}
}

type createPaymentRequest struct {
	PlanName string `json:"plan_name" binding:"required"`
	Amount   int64  `json:"amount"    binding:"required,gt=0"`
}

// CreatePayment handles POST /payment/create
// Requires JWT. Creates a Midtrans Snap token and returns it.
func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	userIDRaw, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID, _ := userIDRaw.(string)

	var req createPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx, err := h.service.CreateSnapToken(userID, req.PlanName, req.Amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create payment"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"snap_token": tx.SnapToken,
		"order_id":   tx.OrderID,
	})
}

// PaymentCallback handles POST /payment/callback
// This endpoint is called by Midtrans (no JWT needed).
func (h *PaymentHandler) PaymentCallback(c *gin.Context) {
	var notification services.MidtransNotification
	if err := c.ShouldBindJSON(&notification); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if err := h.service.HandleNotification(notification); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process notification"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
