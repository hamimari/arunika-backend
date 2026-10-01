package handlers

import (
	"arunika_backend/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AdminUserHandler struct {
	svc *services.AdminUserService
}

func NewAdminUserHandler(svc *services.AdminUserService) *AdminUserHandler {
	return &AdminUserHandler{svc: svc}
}

// GET /admin/users?search=&page=1&per_page=20
func (h *AdminUserHandler) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	users, total, err := h.svc.ListUsers(c.Query("search"), page, perPage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": users, "total": total, "page": page, "per_page": perPage})
}

// GET /admin/users/:id
func (h *AdminUserHandler) GetUserDetail(c *gin.Context) {
	user, sub, err := h.svc.GetUserDetail(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"user":         user,
			"subscription": sub,
		},
	})
}

// PATCH /admin/users/:id/permission
// Body: { "action": "grant"|"revoke", "duration_days": 30 }
// duration_days is optional; 0 or omitted = no expiry when granting.
func (h *AdminUserHandler) UpdatePermission(c *gin.Context) {
	var body struct {
		Action       string `json:"action"        binding:"required"` // "grant" | "revoke"
		DurationDays int    `json:"duration_days"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var err error
	switch body.Action {
	case "grant":
		err = h.svc.GrantPremium(c.Param("id"), body.DurationDays)
	case "revoke":
		err = h.svc.RevokePremium(c.Param("id"))
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be 'grant' or 'revoke'"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "permission updated", "action": body.Action})
}
