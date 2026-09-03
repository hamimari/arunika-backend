package handlers

import (
	"arunika_backend/services"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PaymentHandler struct {
	paymentService      *services.PaymentService
	notificationService *services.NotificationService
	premiumPackService  *services.PremiumPackService
	userService         *services.UserService
}

type CreatePaymentRequest struct {
	PlanName string `json:"plan_name" binding:"required"`
	Amount   int64  `json:"amount" binding:"required"`
}

func NewPaymentHandler(ps *services.PaymentService, ns *services.NotificationService, pp *services.PremiumPackService, us *services.UserService) *PaymentHandler {
	return &PaymentHandler{paymentService: ps, notificationService: ns, premiumPackService: pp, userService: us}
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
	user, _, err := h.userService.GetUserByID(userID.String())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
	}

	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("CreatePayment: invalid input", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	premiumPack, err := h.premiumPackService.GetByName(req.PlanName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid package"})
		return
	}
	snapResp, err := h.paymentService.CreateSnapTransaction(user, premiumPack)
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
