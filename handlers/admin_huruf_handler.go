package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"arunika_backend/models"
	"arunika_backend/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AdminHurufHandler serves /admin/huruf/*.
type AdminHurufHandler struct {
	service *services.HurufAdminService
}

func NewAdminHurufHandler(s *services.HurufAdminService) *AdminHurufHandler {
	return &AdminHurufHandler{service: s}
}

func adminHurufFail(c *gin.Context, err error) {
	var v *services.ValidationFailedError
	switch {
	case errors.As(err, &v):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "draft is not ready to publish", "code": "VALIDATION_FAILED", "fields": v.Fields})
	case errors.Is(err, services.ErrLetterNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "LETTER_NOT_FOUND"})
	case errors.Is(err, services.ErrVersionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "VERSION_NOT_FOUND"})
	case errors.Is(err, services.ErrDraftConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "DRAFT_CONFLICT"})
	case errors.Is(err, services.ErrInvalidOrder):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": "INVALID_ORDER"})
	default:
		slog.Error("admin huruf request failed", "path", c.FullPath(), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process request"})
	}
}

// ids returns the admin and the :id letter, or writes the error.
func (h *AdminHurufHandler) ids(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	adminID, ok := adminIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		adminHurufFail(c, services.ErrLetterNotFound)
		return uuid.Nil, uuid.Nil, false
	}
	return adminID, id, true
}

// List handles GET /admin/huruf/letters.
func (h *AdminHurufHandler) List(c *gin.Context) {
	rows, err := h.service.List()
	if err != nil {
		adminHurufFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// GetDraft handles GET /admin/huruf/letters/:id/draft.
func (h *AdminHurufHandler) GetDraft(c *gin.Context) {
	_, id, ok := h.ids(c)
	if !ok {
		return
	}
	d, err := h.service.GetDraft(id)
	if err != nil {
		adminHurufFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": d})
}

// SaveDraft handles PUT /admin/huruf/letters/:id/draft ({draft, draft_rev}).
func (h *AdminHurufHandler) SaveDraft(c *gin.Context) {
	adminID, id, ok := h.ids(c)
	if !ok {
		return
	}
	var req struct {
		Draft    models.LetterContent `json:"draft"`
		DraftRev *int                 `json:"draft_rev" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	d, err := h.service.SaveDraft(adminID, id, req.Draft, *req.DraftRev)
	if err != nil {
		adminHurufFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": d})
}

// Preview handles POST /admin/huruf/letters/:id/preview (body: a draft).
func (h *AdminHurufHandler) Preview(c *gin.Context) {
	_, id, ok := h.ids(c)
	if !ok {
		return
	}
	var draft models.LetterContent
	if err := c.ShouldBindJSON(&draft); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	v, err := h.service.Preview(id, draft)
	if err != nil {
		adminHurufFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": v})
}

// Publish handles POST /admin/huruf/letters/:id/publish.
func (h *AdminHurufHandler) Publish(c *gin.Context) {
	adminID, id, ok := h.ids(c)
	if !ok {
		return
	}
	v, err := h.service.Publish(adminID, id)
	if err != nil {
		adminHurufFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"version": v.Version, "published_at": v.PublishedAt}})
}

// Versions handles GET /admin/huruf/letters/:id/versions.
func (h *AdminHurufHandler) Versions(c *gin.Context) {
	_, id, ok := h.ids(c)
	if !ok {
		return
	}
	vs, err := h.service.Versions(id)
	if err != nil {
		adminHurufFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": vs})
}

// Rollback handles POST /admin/huruf/letters/:id/rollback/:version.
func (h *AdminHurufHandler) Rollback(c *gin.Context) {
	adminID, id, ok := h.ids(c)
	if !ok {
		return
	}
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil {
		adminHurufFail(c, services.ErrVersionNotFound)
		return
	}
	v, err := h.service.Rollback(adminID, id, version)
	if err != nil {
		adminHurufFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"version": v.Version, "published_at": v.PublishedAt}})
}

func (h *AdminHurufHandler) setHidden(c *gin.Context, hidden bool) {
	adminID, id, ok := h.ids(c)
	if !ok {
		return
	}
	if err := h.service.SetHidden(adminID, id, hidden); err != nil {
		adminHurufFail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Hide handles POST /admin/huruf/letters/:id/hide.
func (h *AdminHurufHandler) Hide(c *gin.Context) { h.setHidden(c, true) }

// Unhide handles POST /admin/huruf/letters/:id/unhide.
func (h *AdminHurufHandler) Unhide(c *gin.Context) { h.setHidden(c, false) }

func (h *AdminHurufHandler) setFree(c *gin.Context, free bool) {
	adminID, id, ok := h.ids(c)
	if !ok {
		return
	}
	if err := h.service.SetFree(adminID, id, free); err != nil {
		adminHurufFail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// SetFree handles POST /admin/huruf/letters/:id/set-free.
func (h *AdminHurufHandler) SetFree(c *gin.Context) { h.setFree(c, true) }

// UnsetFree handles POST /admin/huruf/letters/:id/unset-free.
func (h *AdminHurufHandler) UnsetFree(c *gin.Context) { h.setFree(c, false) }

// Reorder handles PUT /admin/huruf/order ({ids: [...]}).
func (h *AdminHurufHandler) Reorder(c *gin.Context) {
	adminID, ok := adminIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req struct {
		IDs []uuid.UUID `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.Reorder(adminID, req.IDs); err != nil {
		adminHurufFail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
