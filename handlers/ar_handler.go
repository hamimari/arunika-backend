package handlers

import (
	"arunika_backend/services"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"net/http"
	"time"
)

type ArHandler struct {
	service *services.ArService
}

func NewArHandler(s *services.ArService) *ArHandler {
	return &ArHandler{service: s}
}

func (h *ArHandler) FindById(c *gin.Context) {
	id := c.Param("id")
	// Parse before querying: a non-UUID reaching Postgres comes back as a raw
	// driver error (SQLSTATE 22P02), which is both a 500 and a schema leak.
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	content, err := h.service.GetByID(id, optionalUserID(c))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "content not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if content == nil || (content.ExpiresAt != nil && content.ExpiresAt.Before(time.Now())) {
		c.JSON(http.StatusNotFound, gin.H{"error": "content not found"})
		return
	}
	c.JSON(http.StatusOK, content)
}

func (h *ArHandler) GetAll(c *gin.Context) {
	categoryID := c.Query("category_id")
	subCategoryID := c.Query("sub_category_id")
	for name, value := range map[string]string{"category_id": categoryID, "sub_category_id": subCategoryID} {
		if value == "" {
			continue
		}
		if _, err := uuid.Parse(value); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + name})
			return
		}
	}
	cards, err := h.service.GetAll(categoryID, subCategoryID, optionalUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cards})
}

func (h *ArHandler) GetCategories(c *gin.Context) {
	cats, err := h.service.GetAllCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cats})
}
