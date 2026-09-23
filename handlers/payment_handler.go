package handlers

import (
	"arunika_backend/services"
	"encoding/base64"
	"errors"
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

type CreatePlayOrderRequest struct {
	PackageID string `json:"package_id" binding:"required"`
}

type CreatePlayProductOrderRequest struct {
	ProductID string `json:"product_id" binding:"required"`
}

// CreatePlayOrder handles POST /payment/play/create — creates a PENDING
// order for a package the app is about to purchase via Google Play Billing.
func (h *PaymentHandler) CreatePlayOrder(c *gin.Context) {
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

	var req CreatePlayOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("CreatePlayOrder: invalid input", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	pkg, err := h.premiumPackService.GetByID(req.PackageID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid package"})
		return
	}

	order, err := h.paymentService.CreatePlayOrder(user, pkg)
	if err != nil {
		if errors.Is(err, services.ErrPackageNotPlayMapped) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create order"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"order_id":        order.ID,
		"play_product_id": *pkg.PlayProductID,
	}})
}

// CreatePlayProductOrder handles POST /payment/play/create-product — creates a PENDING
// order for a product (AR card or dongeng) the app is about to purchase via Google Play Billing.
func (h *PaymentHandler) CreatePlayProductOrder(c *gin.Context) {
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

	var req CreatePlayProductOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("CreatePlayProductOrder: invalid input", "error", err)
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

	order, err := h.paymentService.CreatePlayProductOrder(user, product)
	if err != nil {
		if errors.Is(err, services.ErrProductNotPlayMapped) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		slog.Error("CreatePlayProductOrder: failed to create order", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create order"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"order_id":        order.ID,
		"play_product_id": *product.PlayProductID,
	}})
}

type VerifyPlayPurchaseRequest struct {
	// OrderID is the order the purchase was started against. Optional: the
	// app omits it when reconciling a purchase Google Play reports as
	// owned but has no in-memory order id for (e.g. it was killed between
	// the purchase completing and reporting it back) — see
	// PaymentService.ResolvePendingPlayOrder.
	OrderID       string `json:"order_id"`
	ProductID     string `json:"product_id" binding:"required"`
	PurchaseToken string `json:"purchase_token" binding:"required"`
}

// VerifyPlayPurchase handles POST /payment/play/verify — verifies a
// completed Google Play Billing purchase and, if valid, settles the order
// and grants entitlement.
func (h *PaymentHandler) VerifyPlayPurchase(c *gin.Context) {
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

	var req VerifyPlayPurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("VerifyPlayPurchase: invalid input", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	var orderID uuid.UUID
	if req.OrderID != "" {
		orderID, err = uuid.Parse(req.OrderID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
			return
		}
	} else {
		orderID, err = h.paymentService.ResolvePendingPlayOrder(userID, req.ProductID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "no pending order found for this purchase"})
			return
		}
	}

	order, err := h.paymentService.VerifyPlayPurchase(c.Request.Context(), orderID, userID, req.PurchaseToken)
	if err != nil {
		slog.Error("VerifyPlayPurchase: verification failed", "order_id", orderID, "error", err)
		switch {
		case errors.Is(err, services.ErrPlayPurchaseTokenReused), errors.Is(err, services.ErrPlayPurchaseNotValid):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, services.ErrPlayBillingNotConfigured):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	if order.Status == "PAID" {
		go h.notificationService.Send(userID, "Pembayaran Berhasil", "Selamat! Akun kamu sudah aktif Premium.", "payment")
	}

	c.JSON(http.StatusOK, gin.H{"data": order})
}

type pubSubPushEnvelope struct {
	Message struct {
		Data string `json:"data"`
	} `json:"message"`
}

// PlayRTDN handles POST /payment/play/rtdn (no JWT — called by Google Cloud
// Pub/Sub push delivery of Real-time Developer Notifications).
func (h *PaymentHandler) PlayRTDN(c *gin.Context) {
	var envelope pubSubPushEnvelope
	if err := c.ShouldBindJSON(&envelope); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	raw, err := base64.StdEncoding.DecodeString(envelope.Message.Data)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message data"})
		return
	}
	if err := h.paymentService.HandlePlayRTDN(c.Request.Context(), raw); err != nil {
		// Logged for investigation, but still acknowledged (200) so Pub/Sub
		// doesn't redeliver indefinitely — an unknown/malformed
		// notification isn't something a retry will fix.
		slog.Error("PlayRTDN: failed to process notification", "error", err)
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type ReportExternalTransactionRequest struct {
	OrderID                  string `json:"order_id" binding:"required"`
	ExternalTransactionToken string `json:"external_transaction_token" binding:"required"`
}

// ReportExternalTransaction handles POST /payment/play/report-external —
// reports a Midtrans-settled order to Google as a User Choice Billing
// external transaction (the user picked Midtrans over Google Play Billing
// in Play's billing choice screen).
func (h *PaymentHandler) ReportExternalTransaction(c *gin.Context) {
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

	var req ReportExternalTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("ReportExternalTransaction: invalid input", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	orderID, err := uuid.Parse(req.OrderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	order, err := h.paymentService.ReportExternalTransaction(c.Request.Context(), orderID, userID, req.ExternalTransactionToken)
	if err != nil {
		if errors.Is(err, services.ErrPlayBillingNotConfigured) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": order})
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
