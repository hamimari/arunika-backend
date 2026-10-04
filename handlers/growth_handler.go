package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"arunika_backend/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GrowthHandler serves /children/:childId/growth. Request and response bodies
// are child health data (UU PDP specific personal data): never log them.
type GrowthHandler struct {
	service *services.GrowthService
}

func NewGrowthHandler(s *services.GrowthService) *GrowthHandler {
	return &GrowthHandler{service: s}
}

// ids returns the caller and path ids, or writes the error response. A
// malformed id answers like a missing one.
func (h *GrowthHandler) ids(c *gin.Context, withMeasurement bool) (userID, childID, id uuid.UUID, ok bool) {
	userID, err := requireUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if childID, err = uuid.Parse(c.Param("childId")); err != nil {
		h.fail(c, services.ErrChildNotFound)
		return
	}
	if withMeasurement {
		if id, err = uuid.Parse(c.Param("id")); err != nil {
			h.fail(c, services.ErrMeasurementNotFound)
			return
		}
	}
	return userID, childID, id, true
}

func (h *GrowthHandler) fail(c *gin.Context, err error) {
	var v *services.GrowthValidationError
	switch {
	case errors.Is(err, services.ErrChildNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "child not found", "code": "CHILD_NOT_FOUND"})
	case errors.Is(err, services.ErrMeasurementNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "measurement not found", "code": "MEASUREMENT_NOT_FOUND"})
	case errors.Is(err, services.ErrClientIDConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "CLIENT_ID_CONFLICT"})
	case errors.As(err, &v):
		body := gin.H{"error": "invalid measurement", "code": v.Code}
		if len(v.Indicators) > 0 {
			body["indicators"] = v.Indicators
		}
		c.JSON(http.StatusUnprocessableEntity, body)
	default:
		slog.Error("growth request failed", "path", c.FullPath(), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process growth request"})
	}
}

// Summary handles GET /children/:childId/growth.
func (h *GrowthHandler) Summary(c *gin.Context) {
	userID, childID, _, ok := h.ids(c, false)
	if !ok {
		return
	}
	sum, err := h.service.Summary(userID, childID)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": sum})
}

// Create handles POST /children/:childId/growth/measurements. A retried
// client_id answers 200 with the row it already created.
func (h *GrowthHandler) Create(c *gin.Context) {
	userID, childID, _, ok := h.ids(c, false)
	if !ok {
		return
	}
	var req services.CreateMeasurementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	view, created, err := h.service.Create(userID, childID, req)
	if err != nil {
		h.fail(c, err)
		return
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	c.JSON(status, gin.H{"data": view})
}

// Update handles PATCH /children/:childId/growth/measurements/:id.
func (h *GrowthHandler) Update(c *gin.Context) {
	userID, childID, id, ok := h.ids(c, true)
	if !ok {
		return
	}
	var req services.UpdateMeasurementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}
	view, err := h.service.Update(userID, childID, id, req)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

// Delete handles DELETE /children/:childId/growth/measurements/:id (soft).
func (h *GrowthHandler) Delete(c *gin.Context) {
	userID, childID, id, ok := h.ids(c, true)
	if !ok {
		return
	}
	if err := h.service.Delete(userID, childID, id); err != nil {
		h.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Restore handles POST /children/:childId/growth/measurements/:id/restore.
func (h *GrowthHandler) Restore(c *gin.Context) {
	userID, childID, id, ok := h.ids(c, true)
	if !ok {
		return
	}
	view, err := h.service.Restore(userID, childID, id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}
