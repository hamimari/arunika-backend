package handlers

import (
	"arunika_backend/models"
	"arunika_backend/services"
	"net/http"
	"strconv"
	"time"

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
// instead of waiting for a webhook/RTDN delivery. Provider-aware: a
// Midtrans order is checked against Midtrans's status API; a Google Play
// order is re-verified using whatever purchase token is already on file
// (see PaymentService.VerifyPlayPurchase) — no manual input needed unless
// no token was ever recorded, in which case RecoverPlayPurchase is the
// fallback.
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

	if order.Provider == models.OrderProviderGooglePlay {
		if order.PurchaseToken == nil || *order.PurchaseToken == "" {
			c.JSON(http.StatusConflict, gin.H{"error": "no purchase token on file yet for this order — use recover-play with a token"})
			return
		}
		updated, err := h.paymentSvc.VerifyPlayPurchase(c.Request.Context(), order.ID, order.UserID, *order.PurchaseToken)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		order = updated
	} else {
		order = h.paymentSvc.SyncOrderStatus(order)
	}

	views, err := h.svc.EnrichOne(*order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": views})
}

// POST /admin/orders/:id/recover-play — manually settles a Google Play
// purchase against a purchase token an admin obtained out-of-band (e.g. a
// support case where a user's order got stuck PENDING because the app
// never called verify), reusing the same idempotent/replay-safe path a
// normal purchase completes through.
func (h *AdminOrderHandler) RecoverPlayPurchase(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	var req struct {
		PurchaseToken string `json:"purchase_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	order, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	updated, err := h.paymentSvc.VerifyPlayPurchase(c.Request.Context(), id, order.UserID, req.PurchaseToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	views, err := h.svc.EnrichOne(*updated)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": views})
}

// reconcilePlayLookbackDays is kept a day short of Google's actual 30-day
// limit on purchases.voidedpurchases.list's startTime — an exact "30 days
// ago" computed here lands slightly further back by the time the request
// reaches Google (network latency, clock drift), which Google's own
// boundary check rejects with "Start time must be within 30 days".
const reconcilePlayLookbackDays = 29

// POST /admin/orders/reconcile-play — polls Google Play's Voided Purchases
// API for the last 29 days and revokes entitlement for any PAID order
// whose purchase Google has since refunded/canceled/charged-back,
// including its own automatic refund of a purchase left unacknowledged for
// 3 days. Runs automatically on a schedule too (see main.go); this lets an
// admin trigger the same reconciliation on demand.
func (h *AdminOrderHandler) ReconcilePlayPurchases(c *gin.Context) {
	count, err := h.paymentSvc.ReconcileVoidedPurchases(c.Request.Context(), time.Now().AddDate(0, 0, -reconcilePlayLookbackDays))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"reconciled": count}})
}
