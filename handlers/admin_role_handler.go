package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"arunika_backend/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AdminRoleHandler serves role management and the audit log.
type AdminRoleHandler struct {
	roles *services.AdminRoleService
	audit *services.AuditService
}

func NewAdminRoleHandler(roles *services.AdminRoleService, audit *services.AuditService) *AdminRoleHandler {
	return &AdminRoleHandler{roles: roles, audit: audit}
}

// adminIDFrom returns the authenticated admin's id (set by AdminAuthMiddleware).
func adminIDFrom(c *gin.Context) (uuid.UUID, bool) {
	raw, _ := c.Get("adminID")
	s, _ := raw.(string)
	id, err := uuid.Parse(s)
	return id, err == nil
}

// List handles GET /admin/admins.
func (h *AdminRoleHandler) List(c *gin.Context) {
	admins, err := h.roles.List()
	if err != nil {
		slog.Error("list admins", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list admins"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": admins})
}

// SetRole handles PATCH /admin/admins/:id/role.
func (h *AdminRoleHandler) SetRole(c *gin.Context) {
	actor, ok := adminIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	target, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "admin not found", "code": "ADMIN_NOT_FOUND"})
		return
	}
	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	view, err := h.roles.SetRole(actor, target, req.Role)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"data": view})
	case errors.Is(err, services.ErrAdminNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "ADMIN_NOT_FOUND"})
	case errors.Is(err, services.ErrInvalidRole):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": "INVALID_ROLE"})
	case errors.Is(err, services.ErrLastPublisher):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "LAST_PUBLISHER"})
	default:
		slog.Error("set admin role", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to change role"})
	}
}

// AuditLog handles GET /admin/audit-log?entity_type=&entity_id=&page=&per_page=.
func (h *AdminRoleHandler) AuditLog(c *gin.Context) {
	page, perPage := pageParams(c)
	entries, total, err := h.audit.List(c.Query("entity_type"), c.Query("entity_id"), page, perPage)
	if err != nil {
		slog.Error("list audit log", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list audit log"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entries, "total": total, "page": page, "per_page": perPage})
}
