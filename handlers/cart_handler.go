package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"arunika_backend/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CartHandler serves the Keranjang Belanja endpoints: the per-account
// cart, checkout into an order, and the purchase verification of cart
// orders.
type CartHandler struct {
	cart     *services.CartService
	payments *services.PaymentService
}

func NewCartHandler(cart *services.CartService, payments *services.PaymentService) *CartHandler {
	return &CartHandler{cart: cart, payments: payments}
}

// cartErrorStatus maps a cart rule error to its HTTP status and code.
func cartErrorStatus(err error) (int, string, bool) {
	for _, e := range []error{
		services.ErrCartAlreadyOwned, services.ErrCartNotForSale, services.ErrCartFreeItem,
		services.ErrCartSubscription, services.ErrCartFull, services.ErrCartTotalLimit,
		services.ErrCartEmpty, services.ErrCartAmountUnavailable,
	} {
		if errors.Is(err, e) {
			return http.StatusConflict, e.Error(), true
		}
	}
	return 0, "", false
}

func (h *CartHandler) respondErr(c *gin.Context, op string, err error) {
	if status, code, ok := cartErrorStatus(err); ok {
		c.JSON(status, gin.H{"error": code, "code": code})
		return
	}
	slog.Error(op, "error", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

func (h *CartHandler) requireEnabled(c *gin.Context) bool {
	if h.cart.Enabled() {
		return true
	}
	c.JSON(http.StatusForbidden, gin.H{"error": "CART_DISABLED", "code": "CART_DISABLED"})
	return false
}

// Get handles GET /cart.
func (h *CartHandler) Get(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	view, err := h.cart.Get(userID)
	if err != nil {
		h.respondErr(c, "GetCart", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

type addCartItemRequest struct {
	ProductID string `json:"product_id" binding:"required"`
}

// AddItem handles POST /cart/items. Adding an item already in the cart is
// a no-op; undo of a removal is this call again.
func (h *CartHandler) AddItem(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if !h.requireEnabled(c) {
		return
	}
	var req addCartItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}
	view, err := h.cart.Add(userID, productID)
	if err != nil {
		h.respondErr(c, "AddCartItem", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

// RemoveItem handles DELETE /cart/items/:product_id.
func (h *CartHandler) RemoveItem(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	productID, err := uuid.Parse(c.Param("product_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
		return
	}
	view, err := h.cart.Remove(userID, productID)
	if err != nil {
		h.respondErr(c, "RemoveCartItem", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

// Clear handles DELETE /cart ("Hapus semua").
func (h *CartHandler) Clear(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.cart.Clear(userID); err != nil {
		h.respondErr(c, "ClearCart", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": services.CartView{Items: []services.CartItemView{}, Notices: []services.CartNotice{}}})
}

type createOrderRequest struct {
	Store string `json:"store"`
	// Items, when set, buys exactly these products (single "Beli") and
	// skips the cart.
	Items []struct {
		ProductID string `json:"product_id"`
	} `json:"items"`
}

// CreateOrder handles POST /orders: freezes the cart (or the given items)
// into an order and returns the store product to buy. Answers 409
// CART_CHANGED with the new cart when the parent must look again.
func (h *CartHandler) CreateOrder(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	if req.Store != "" && req.Store != "google" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported store"})
		return
	}
	var productIDs []uuid.UUID
	if len(req.Items) > 0 {
		for _, it := range req.Items {
			id, err := uuid.Parse(it.ProductID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
				return
			}
			productIDs = append(productIDs, id)
		}
	} else if !h.requireEnabled(c) {
		return
	}
	order, err := h.cart.CreateOrder(userID, productIDs, c.GetHeader("Idempotency-Key"))
	if err != nil {
		var changed *services.CartChangedError
		if errors.As(err, &changed) {
			c.JSON(http.StatusConflict, gin.H{"error": "CART_CHANGED", "code": "CART_CHANGED", "data": changed.Cart})
			return
		}
		h.respondErr(c, "CreateOrder", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": order})
}

type verifyOrderRequest struct {
	PurchaseToken string `json:"purchase_token" binding:"required"`
}

// VerifyOrder handles POST /orders/:id/verify: settles a cart order with
// its Google Play purchase. Returns the order phase: diberikan when
// granted, diproses while Google is still taking the payment.
func (h *CartHandler) VerifyOrder(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}
	var req verifyOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	order, err := h.payments.VerifyPlayPurchase(c.Request.Context(), orderID, userID, req.PurchaseToken)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"order_id": order.ID, "phase": services.OrderPhase(order)}})
	case errors.Is(err, services.ErrPlayPurchasePending):
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"order_id": orderID, "phase": services.PhaseProcessing}})
	case errors.Is(err, services.ErrPlayPurchaseMismatch), errors.Is(err, services.ErrPlayPurchaseNotValid),
		errors.Is(err, services.ErrPlayPurchaseTokenReused):
		slog.Error("VerifyOrder: purchase rejected", "order_id", orderID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrPlayBillingNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
	default:
		// The token is on file; the worker keeps retrying the grant.
		slog.Error("VerifyOrder: settle failed", "order_id", orderID, "error", err)
		c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"order_id": orderID, "phase": services.PhaseProcessing}})
	}
}

// Entitlements handles GET /entitlements: the account's owned single items,
// which back "Pulihkan pembelian".
func (h *CartHandler) Entitlements(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	ids, err := h.cart.OwnedProducts(userID)
	if err != nil {
		h.respondErr(c, "Entitlements", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"product_ids": ids}})
}
