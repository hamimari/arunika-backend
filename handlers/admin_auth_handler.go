package handlers

import (
	"arunika_backend/services"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type AdminAuthHandler struct {
	svc *services.AdminAuthService
}

func NewAdminAuthHandler(svc *services.AdminAuthService) *AdminAuthHandler {
	return &AdminAuthHandler{svc: svc}
}

// POST /admin/auth/login
func (h *AdminAuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"    binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accessToken, refreshToken, err := h.svc.Login(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// POST /admin/auth/refresh
func (h *AdminAuthHandler) Refresh(c *gin.Context) {
	var req struct {
		AdminID      string `json:"admin_id"      binding:"required"`
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accessToken, err := h.svc.RefreshToken(req.AdminID, req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"access_token": accessToken})
}

// POST /admin/auth/logout
func (h *AdminAuthHandler) Logout(c *gin.Context) {
	jti, _ := c.Get("jti")
	refreshToken, _ := c.Get("refresh_token")
	expiry, _ := c.Get("exp")

	jtiStr, _ := jti.(string)
	rtStr, _ := refreshToken.(string)
	exp, _ := expiry.(time.Time)

	if err := h.svc.Logout(jtiStr, rtStr, exp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "logout failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}
