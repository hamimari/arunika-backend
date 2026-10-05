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

// AdminAngkaHandler serves /admin/angka/*.
type AdminAngkaHandler struct {
	service *services.AngkaAdminService
}

func NewAdminAngkaHandler(s *services.AngkaAdminService) *AdminAngkaHandler {
	return &AdminAngkaHandler{service: s}
}

func adminAngkaFail(c *gin.Context, err error) {
	var v *services.ValidationFailedError
	switch {
	case errors.As(err, &v):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "draft is not ready to publish", "code": "VALIDATION_FAILED", "fields": v.Fields})
	case errors.Is(err, services.ErrAngkaObjectNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "OBJECT_NOT_FOUND"})
	case errors.Is(err, services.ErrAngkaNumberNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "NUMBER_NOT_FOUND"})
	case errors.Is(err, services.ErrAngkaLevelNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "LEVEL_NOT_FOUND"})
	case errors.Is(err, services.ErrVersionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "VERSION_NOT_FOUND"})
	case errors.Is(err, services.ErrDraftConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "DRAFT_CONFLICT"})
	case errors.Is(err, services.ErrObjectInUse):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "OBJECT_IN_USE"})
	case errors.Is(err, services.ErrObjectLastInLevel):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "OBJECT_LAST_IN_LEVEL"})
	case errors.Is(err, services.ErrLevelInUse):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "LEVEL_IN_USE"})
	case errors.Is(err, services.ErrInvalidOrder):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": "INVALID_ORDER"})
	default:
		slog.Error("admin angka request failed", "path", c.FullPath(), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process request"})
	}
}

func (h *AdminAngkaHandler) admin(c *gin.Context) (uuid.UUID, bool) {
	adminID, ok := adminIDFrom(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	}
	return adminID, ok
}

// idParam returns the admin and the :id, answering notFound when it isn't a
// UUID.
func (h *AdminAngkaHandler) idParam(c *gin.Context, notFound error) (uuid.UUID, uuid.UUID, bool) {
	adminID, ok := h.admin(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		adminAngkaFail(c, notFound)
		return uuid.Nil, uuid.Nil, false
	}
	return adminID, id, true
}

func (h *AdminAngkaHandler) valueParam(c *gin.Context) (uuid.UUID, int, bool) {
	adminID, ok := h.admin(c)
	if !ok {
		return uuid.Nil, 0, false
	}
	v, err := strconv.Atoi(c.Param("value"))
	if err != nil {
		adminAngkaFail(c, services.ErrAngkaNumberNotFound)
		return uuid.Nil, 0, false
	}
	return adminID, v, true
}

func angkaOK(c *gin.Context, data interface{}, err error) {
	if err != nil {
		adminAngkaFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func angkaNoContent(c *gin.Context, err error) {
	if err != nil {
		adminAngkaFail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type draftRequest[T any] struct {
	Draft    T    `json:"draft"`
	DraftRev *int `json:"draft_rev" binding:"required"`
}

// ─── Objects ────────────────────────────────────────────────────────────────

// ListObjects handles GET /admin/angka/objects.
func (h *AdminAngkaHandler) ListObjects(c *gin.Context) {
	rows, err := h.service.ListObjects()
	angkaOK(c, rows, err)
}

// CreateObject handles POST /admin/angka/objects ({name?}).
func (h *AdminAngkaHandler) CreateObject(c *gin.Context) {
	adminID, isAdmin := h.admin(c)
	if !isAdmin {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	_ = c.ShouldBindJSON(&req)
	d, err := h.service.CreateObject(adminID, req.Name)
	if err != nil {
		adminAngkaFail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": d})
}

// GetObjectDraft handles GET /admin/angka/objects/:id/draft.
func (h *AdminAngkaHandler) GetObjectDraft(c *gin.Context) {
	_, id, isOK := h.idParam(c, services.ErrAngkaObjectNotFound)
	if !isOK {
		return
	}
	d, err := h.service.GetObjectDraft(id)
	angkaOK(c, d, err)
}

// SaveObjectDraft handles PUT /admin/angka/objects/:id/draft.
func (h *AdminAngkaHandler) SaveObjectDraft(c *gin.Context) {
	adminID, id, isOK := h.idParam(c, services.ErrAngkaObjectNotFound)
	if !isOK {
		return
	}
	var req draftRequest[models.AngkaObjectContent]
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	d, err := h.service.SaveObjectDraft(adminID, id, req.Draft, *req.DraftRev)
	angkaOK(c, d, err)
}

// PublishObject handles POST /admin/angka/objects/:id/publish.
func (h *AdminAngkaHandler) PublishObject(c *gin.Context) {
	adminID, id, isOK := h.idParam(c, services.ErrAngkaObjectNotFound)
	if isOK {
		angkaNoContent(c, h.service.PublishObject(adminID, id))
	}
}

// HideObject handles POST /admin/angka/objects/:id/hide.
func (h *AdminAngkaHandler) HideObject(c *gin.Context) { h.objectHidden(c, true) }

// UnhideObject handles POST /admin/angka/objects/:id/unhide.
func (h *AdminAngkaHandler) UnhideObject(c *gin.Context) { h.objectHidden(c, false) }

func (h *AdminAngkaHandler) objectHidden(c *gin.Context, hidden bool) {
	adminID, id, isOK := h.idParam(c, services.ErrAngkaObjectNotFound)
	if isOK {
		angkaNoContent(c, h.service.SetObjectHidden(adminID, id, hidden))
	}
}

// DeleteObject handles DELETE /admin/angka/objects/:id.
func (h *AdminAngkaHandler) DeleteObject(c *gin.Context) {
	adminID, id, isOK := h.idParam(c, services.ErrAngkaObjectNotFound)
	if isOK {
		angkaNoContent(c, h.service.DeleteObject(adminID, id))
	}
}

// ─── Numbers ────────────────────────────────────────────────────────────────

// ListNumbers handles GET /admin/angka/numbers.
func (h *AdminAngkaHandler) ListNumbers(c *gin.Context) {
	rows, err := h.service.ListNumbers()
	angkaOK(c, rows, err)
}

// SaveNumberDraft handles PUT /admin/angka/numbers/:value/draft.
func (h *AdminAngkaHandler) SaveNumberDraft(c *gin.Context) {
	adminID, value, isOK := h.valueParam(c)
	if !isOK {
		return
	}
	var req draftRequest[models.AngkaNumberContent]
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	d, err := h.service.SaveNumberDraft(adminID, value, req.Draft, *req.DraftRev)
	angkaOK(c, d, err)
}

// PublishNumber handles POST /admin/angka/numbers/:value/publish.
func (h *AdminAngkaHandler) PublishNumber(c *gin.Context) {
	adminID, value, isOK := h.valueParam(c)
	if isOK {
		angkaNoContent(c, h.service.PublishNumber(adminID, value))
	}
}

// HideNumber handles POST /admin/angka/numbers/:value/hide.
func (h *AdminAngkaHandler) HideNumber(c *gin.Context) { h.numberHidden(c, true) }

// UnhideNumber handles POST /admin/angka/numbers/:value/unhide.
func (h *AdminAngkaHandler) UnhideNumber(c *gin.Context) { h.numberHidden(c, false) }

func (h *AdminAngkaHandler) numberHidden(c *gin.Context, hidden bool) {
	adminID, value, isOK := h.valueParam(c)
	if isOK {
		angkaNoContent(c, h.service.SetNumberHidden(adminID, value, hidden))
	}
}

// SetNumberFree handles POST /admin/angka/numbers/:value/set-free.
func (h *AdminAngkaHandler) SetNumberFree(c *gin.Context) { h.numberFree(c, true) }

// UnsetNumberFree handles POST /admin/angka/numbers/:value/unset-free.
func (h *AdminAngkaHandler) UnsetNumberFree(c *gin.Context) { h.numberFree(c, false) }

func (h *AdminAngkaHandler) numberFree(c *gin.Context, free bool) {
	adminID, value, isOK := h.valueParam(c)
	if isOK {
		angkaNoContent(c, h.service.SetNumberFree(adminID, value, free))
	}
}

// ─── Levels ─────────────────────────────────────────────────────────────────

// ListLevels handles GET /admin/angka/levels.
func (h *AdminAngkaHandler) ListLevels(c *gin.Context) {
	rows, err := h.service.ListLevels()
	angkaOK(c, rows, err)
}

// CreateLevel handles POST /admin/angka/levels.
func (h *AdminAngkaHandler) CreateLevel(c *gin.Context) {
	adminID, isAdmin := h.admin(c)
	if !isAdmin {
		return
	}
	d, err := h.service.CreateLevel(adminID)
	if err != nil {
		adminAngkaFail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": d})
}

// GetLevelDraft handles GET /admin/angka/levels/:id/draft.
func (h *AdminAngkaHandler) GetLevelDraft(c *gin.Context) {
	_, id, isOK := h.idParam(c, services.ErrAngkaLevelNotFound)
	if !isOK {
		return
	}
	d, err := h.service.GetLevelDraft(id)
	angkaOK(c, d, err)
}

// SaveLevelDraft handles PUT /admin/angka/levels/:id/draft.
func (h *AdminAngkaHandler) SaveLevelDraft(c *gin.Context) {
	adminID, id, isOK := h.idParam(c, services.ErrAngkaLevelNotFound)
	if !isOK {
		return
	}
	var req draftRequest[models.AngkaLevelContent]
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	d, err := h.service.SaveLevelDraft(adminID, id, req.Draft, *req.DraftRev)
	angkaOK(c, d, err)
}

// PreviewLevel handles POST /admin/angka/levels/:id/preview ({draft, seed}).
func (h *AdminAngkaHandler) PreviewLevel(c *gin.Context) {
	_, id, isOK := h.idParam(c, services.ErrAngkaLevelNotFound)
	if !isOK {
		return
	}
	var req struct {
		Draft models.AngkaLevelContent `json:"draft"`
		Seed  uint32                   `json:"seed"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := h.service.PreviewLevel(id, req.Draft, req.Seed)
	angkaOK(c, p, err)
}

// LevelVersions handles GET /admin/angka/levels/:id/versions.
func (h *AdminAngkaHandler) LevelVersions(c *gin.Context) {
	_, id, isOK := h.idParam(c, services.ErrAngkaLevelNotFound)
	if !isOK {
		return
	}
	vs, err := h.service.LevelVersions(id)
	angkaOK(c, vs, err)
}

// PublishLevel handles POST /admin/angka/levels/:id/publish.
func (h *AdminAngkaHandler) PublishLevel(c *gin.Context) {
	adminID, id, isOK := h.idParam(c, services.ErrAngkaLevelNotFound)
	if !isOK {
		return
	}
	v, err := h.service.PublishLevel(adminID, id)
	if err != nil {
		adminAngkaFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"version": v.Version, "published_at": v.PublishedAt}})
}

// RollbackLevel handles POST /admin/angka/levels/:id/rollback/:version.
func (h *AdminAngkaHandler) RollbackLevel(c *gin.Context) {
	adminID, id, isOK := h.idParam(c, services.ErrAngkaLevelNotFound)
	if !isOK {
		return
	}
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil {
		adminAngkaFail(c, services.ErrVersionNotFound)
		return
	}
	v, err := h.service.RollbackLevel(adminID, id, version)
	if err != nil {
		adminAngkaFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"version": v.Version, "published_at": v.PublishedAt}})
}

// HideLevel handles POST /admin/angka/levels/:id/hide.
func (h *AdminAngkaHandler) HideLevel(c *gin.Context) { h.levelHidden(c, true) }

// UnhideLevel handles POST /admin/angka/levels/:id/unhide.
func (h *AdminAngkaHandler) UnhideLevel(c *gin.Context) { h.levelHidden(c, false) }

func (h *AdminAngkaHandler) levelHidden(c *gin.Context, hidden bool) {
	adminID, id, isOK := h.idParam(c, services.ErrAngkaLevelNotFound)
	if isOK {
		angkaNoContent(c, h.service.SetLevelHidden(adminID, id, hidden))
	}
}

// SetLevelFree handles POST /admin/angka/levels/:id/set-free.
func (h *AdminAngkaHandler) SetLevelFree(c *gin.Context) { h.levelFree(c, true) }

// UnsetLevelFree handles POST /admin/angka/levels/:id/unset-free.
func (h *AdminAngkaHandler) UnsetLevelFree(c *gin.Context) { h.levelFree(c, false) }

func (h *AdminAngkaHandler) levelFree(c *gin.Context, free bool) {
	adminID, id, isOK := h.idParam(c, services.ErrAngkaLevelNotFound)
	if isOK {
		angkaNoContent(c, h.service.SetLevelFree(adminID, id, free))
	}
}

// ReorderLevels handles PUT /admin/angka/levels/order ({ids: [...]}).
func (h *AdminAngkaHandler) ReorderLevels(c *gin.Context) {
	adminID, isAdmin := h.admin(c)
	if !isAdmin {
		return
	}
	var req struct {
		IDs []uuid.UUID `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	angkaNoContent(c, h.service.ReorderLevels(adminID, req.IDs))
}

// DeleteLevel handles DELETE /admin/angka/levels/:id.
func (h *AdminAngkaHandler) DeleteLevel(c *gin.Context) {
	adminID, id, isOK := h.idParam(c, services.ErrAngkaLevelNotFound)
	if isOK {
		angkaNoContent(c, h.service.DeleteLevel(adminID, id))
	}
}
