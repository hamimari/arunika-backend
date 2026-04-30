package handlers

import (
	"arunika_backend/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AdminAnalyticsHandler struct {
	svc *services.AdminAnalyticsService
}

func NewAdminAnalyticsHandler(svc *services.AdminAnalyticsService) *AdminAnalyticsHandler {
	return &AdminAnalyticsHandler{svc: svc}
}

// GET /admin/analytics/dau?days=30
func (h *AdminAnalyticsHandler) GetDAU(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 || days > 365 {
		days = 30
	}
	rows, err := h.svc.GetDAU(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// GET /admin/analytics/new-users?days=30
func (h *AdminAnalyticsHandler) GetNewUsers(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 || days > 365 {
		days = 30
	}
	rows, err := h.svc.GetNewUsers(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// GET /admin/analytics/popular-features
func (h *AdminAnalyticsHandler) GetPopularFeatures(c *gin.Context) {
	rows, err := h.svc.GetPopularFeatures()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// GET /admin/analytics/payments?from=2024-01-01&to=2024-01-31
func (h *AdminAnalyticsHandler) GetPaymentMetrics(c *gin.Context) {
	rows, err := h.svc.GetPaymentMetrics(c.Query("from"), c.Query("to"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// GET /admin/analytics/subscription-stats
func (h *AdminAnalyticsHandler) GetSubscriptionStats(c *gin.Context) {
	stats, err := h.svc.GetSubscriptionStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}
