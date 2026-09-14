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
	productService      *services.ProductService
}

type CreatePaymentRequest struct {
	PlanName string `json:"plan_name" binding:"required"`
	Amount   int64  `json:"amount" binding:"required"`
}

type CreateProductPaymentRequest struct {
	ProductID string `json:"product_id" binding:"required"`
}

func NewPaymentHandler(ps *services.PaymentService, ns *services.NotificationService, pp *services.PremiumPackService, us *services.UserService, prs *services.ProductService) *PaymentHandler {
	return &PaymentHandler{paymentService: ps, notificationService: ns, premiumPackService: pp, userService: us, productService: prs}
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
	user, _, _, err := h.userService.GetUserByID(userID.String())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
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

// CreateProductTransaction handles POST /payment/create-product (single-product checkout)
func (h *PaymentHandler) CreateProductTransaction(c *gin.Context) {
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
	user, _, _, err := h.userService.GetUserByID(userID.String())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var req CreateProductPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("CreateProductPayment: invalid input", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}
	product, err := h.productService.GetByID(productID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product"})
		return
	}
	itemName, err := h.productService.ResolveDisplayName(productID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve product"})
		return
	}

	snapResp, err := h.paymentService.CreateSnapTransactionForProduct(user, product, itemName)
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
