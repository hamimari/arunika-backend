package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"arunika_backend/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AngkaHandler serves the app's Belajar Angka routes.
type AngkaHandler struct {
	service *services.AngkaService
}

func NewAngkaHandler(s *services.AngkaService) *AngkaHandler {
	return &AngkaHandler{service: s}
}

func angkaFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrPremiumRequired):
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "premium subscription required", "code": "PREMIUM_REQUIRED"})
	case errors.Is(err, services.ErrLevelLocked):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "LEVEL_LOCKED"})
	case errors.Is(err, services.ErrAngkaNumberNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "NUMBER_NOT_FOUND"})
	case errors.Is(err, services.ErrAngkaLevelNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "LEVEL_NOT_FOUND"})
	case errors.Is(err, services.ErrAngkaSessionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error(), "code": "SESSION_NOT_FOUND"})
	case errors.Is(err, services.ErrChildNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "child not found", "code": "CHILD_NOT_FOUND"})
	case errors.Is(err, services.ErrInvalidTry):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": "INVALID_TRY"})
	case errors.Is(err, services.ErrSessionIncomplete):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": "SESSION_INCOMPLETE"})
	default:
		slog.Error("angka request failed", "path", c.FullPath(), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process request"})
	}
}

// Manifest handles GET /learn/angka/manifest, answering 304 to a matching
// If-None-Match.
func (h *AngkaHandler) Manifest(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	m, etag, err := h.service.Manifest(userID)
	if err != nil {
		angkaFail(c, err)
		return
	}
	c.Header("ETag", etag)
	c.Header("Cache-Control", "private, no-cache")
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": m})
}

// Number handles GET /learn/angka/numbers/:value.
func (h *AngkaHandler) Number(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	value, err := strconv.Atoi(c.Param("value"))
	if err != nil {
		angkaFail(c, services.ErrAngkaNumberNotFound)
		return
	}
	v, err := h.service.Number(userID, value)
	if err != nil {
		angkaFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": v})
}

// Level handles GET /learn/angka/levels/:id.
func (h *AngkaHandler) Level(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		angkaFail(c, services.ErrAngkaLevelNotFound)
		return
	}
	v, err := h.service.Level(userID, id)
	if err != nil {
		angkaFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": v})
}

// Progress handles GET /children/:childId/angka/progress.
func (h *AngkaHandler) Progress(c *gin.Context) {
	userID, childID, ok := childParam(c)
	if !ok {
		return
	}
	rows, err := h.service.Progress(userID, childID)
	if err != nil {
		angkaFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// StartSession handles POST /children/:childId/angka/sessions
// ({level_id, restart?}).
func (h *AngkaHandler) StartSession(c *gin.Context) {
	userID, childID, ok := childParam(c)
	if !ok {
		return
	}
	var req struct {
		LevelID uuid.UUID `json:"level_id" binding:"required"`
		Restart bool      `json:"restart"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	v, err := h.service.StartSession(userID, childID, req.LevelID, req.Restart)
	if err != nil {
		angkaFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": v})
}

func (h *AngkaHandler) session(c *gin.Context) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	userID, childID, ok := childParam(c)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		angkaFail(c, services.ErrAngkaSessionNotFound)
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return userID, childID, id, true
}

// RecordTry handles PATCH /children/:childId/angka/sessions/:id
// ({q, try, answer}).
func (h *AngkaHandler) RecordTry(c *gin.Context) {
	userID, childID, id, ok := h.session(c)
	if !ok {
		return
	}
	var req struct {
		Q      *int `json:"q" binding:"required"`
		Try    *int `json:"try" binding:"required"`
		Answer *int `json:"answer" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	v, err := h.service.RecordTry(userID, childID, id, *req.Q, *req.Try, *req.Answer)
	if err != nil {
		angkaFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": v})
}

// CompleteSession handles POST /children/:childId/angka/sessions/:id/complete.
func (h *AngkaHandler) CompleteSession(c *gin.Context) {
	userID, childID, id, ok := h.session(c)
	if !ok {
		return
	}
	v, err := h.service.CompleteSession(userID, childID, id)
	if err != nil {
		angkaFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": v})
}
