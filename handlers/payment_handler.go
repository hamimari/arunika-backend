package handlers

import (
	"arunika_backend/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
)

type PaymentHandler struct {
	paymentService      *services.PaymentService
	notificationService *services.NotificationService
}

func NewPaymentHandler(ps *services.PaymentService, ns *services.NotificationService) *PaymentHandler {
	return &PaymentHandler{paymentService: ps, notificationService: ns}
}

// CreateTransaction handles POST /payment/create
func (h *PaymentHandler) CreateTransaction(c *gin.Context) {
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

	snapResp, err := h.paymentService.CreateSnapTransaction(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create payment"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": snapResp})
}

// Webhook handles POST /payment/webhook (no JWT — called by Midtrans)
func (h *PaymentHandler) Webhook(c *gin.Context) {
	var notif services.WebhookNotification
	if err := c.ShouldBindJSON(&notif); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := h.paymentService.HandleWebhook(notif)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Dispatch payment result notifications based on transaction status.
	if userID != uuid.Nil {
		switch notif.TransactionStatus {
		case "settlement", "capture":
			go h.notificationService.Send(userID, "Pembayaran Berhasil", "Selamat! Akun kamu sudah aktif Premium.", "payment")
		case "deny", "expire", "cancel":
			go h.notificationService.Send(userID, "Pembayaran Gagal", "Maaf, pembayaran kamu tidak berhasil. Silakan coba lagi.", "payment")
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
