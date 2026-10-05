package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"arunika_backend/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// HurufHandler serves the app's Belajar Huruf routes.
type HurufHandler struct {
	service *services.HurufService
}

func NewHurufHandler(s *services.HurufService) *HurufHandler {
	return &HurufHandler{service: s}
}

func hurufFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrPremiumRequired):
		c.JSON(http.StatusPaymentRequired, gin.H{"error": "premium subscription required", "code": "PREMIUM_REQUIRED"})
	case errors.Is(err, services.ErrLetterNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "letter not found", "code": "LETTER_NOT_FOUND"})
	case errors.Is(err, services.ErrChildNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "child not found", "code": "CHILD_NOT_FOUND"})
	case errors.Is(err, services.ErrInvalidScore):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "code": "INVALID_SCORE"})
	default:
		slog.Error("huruf request failed", "path", c.FullPath(), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process request"})
	}
}

// Manifest handles GET /learn/huruf/manifest, answering 304 to a matching
// If-None-Match.
func (h *HurufHandler) Manifest(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	m, etag, err := h.service.Manifest(userID)
	if err != nil {
		hurufFail(c, err)
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

// Letter handles GET /learn/huruf/letters/:id.
func (h *HurufHandler) Letter(c *gin.Context) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		hurufFail(c, services.ErrLetterNotFound)
		return
	}
	v, err := h.service.Letter(userID, id)
	if err != nil {
		hurufFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": v})
}

func childParam(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return uuid.Nil, uuid.Nil, false
	}
	childID, err := uuid.Parse(c.Param("childId"))
	if err != nil {
		hurufFail(c, services.ErrChildNotFound)
		return uuid.Nil, uuid.Nil, false
	}
	return userID, childID, true
}

// Progress handles GET /children/:childId/huruf/progress.
func (h *HurufHandler) Progress(c *gin.Context) {
	userID, childID, ok := childParam(c)
	if !ok {
		return
	}
	rows, err := h.service.Progress(userID, childID)
	if err != nil {
		hurufFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows})
}

// UpdateProgress handles PUT /children/:childId/huruf/progress/:letterId.
func (h *HurufHandler) UpdateProgress(c *gin.Context) {
	userID, childID, ok := childParam(c)
	if !ok {
		return
	}
	letterID, err := uuid.Parse(c.Param("letterId"))
	if err != nil {
		hurufFail(c, services.ErrLetterNotFound)
		return
	}
	var req services.ProgressUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row, err := h.service.UpdateProgress(userID, childID, letterID, req)
	if err != nil {
		hurufFail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}
