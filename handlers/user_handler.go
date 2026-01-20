package handlers

import (
	"arunika_backend/models"
	"arunika_backend/services"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"log"
	"net/http"
	"time"
)

type UpdateUserRequest struct {
	ID           uuid.UUID     `json:"id" binding:"required"`
	Name         string        `json:"name" binding:"required"`
	PhoneNumber  string        `json:"phone_number" binding:"required"`
	EmailAddress string        `json:"email_address" binding:"required"`
	Address      string        `json:"address" binding:"required"`
	City         string        `json:"city" binding:"required"`
	Child        []UpdateChild `json:"child" binding:"required"`
}

type UpdateChild struct {
	ID        uuid.UUID `json:"id" binding:"required"`
	Name      string    `json:"name" binding:"required"`
	Gender    string    `json:"gender" binding:"required"`
	BirthDate string    `json:"date_of_birth" binding:"required"`
}

type UserHandler struct {
	service *services.UserService
}

func NewUserHandler(s *services.UserService) *UserHandler {
	return &UserHandler{service: s}
}

func (h *UserHandler) GetUserByID(c *gin.Context) {
	tokenUserID, _ := c.Get("userID")
	id := c.Param("id")

	if tokenUserID != id {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
		return
	}

	user, err := h.service.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": user})
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Println("error: ", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
		return
	}

	parent := &models.Parent{
		BaseModel: models.BaseModel{
			ID:        req.ID,
			UpdatedAt: time.Now(),
		},
		Name:         req.Name,
		PhoneNumber:  req.PhoneNumber,
		EmailAddress: req.EmailAddress,
		City:         req.City,
		Address:      req.Address,
	}

	//layout := "2006-01-02T15:04:05.000"
	for _, ch := range req.Child {
		time.Parse(time.RFC3339, ch.BirthDate)
		dob, err := time.Parse(time.RFC3339, ch.BirthDate)
		if err != nil {
			log.Println("error2: ", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid birth date input"})
			return
		}
		parent.Children = append(parent.Children, models.Children{
			BaseModel: models.BaseModel{
				ID:        ch.ID,
				UpdatedAt: time.Now(),
			},
			Name:        ch.Name,
			Gender:      ch.Gender,
			DateOfBirth: dob,
		})
	}

	updated, err := h.service.UpdateUser(parent)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": updated})
}
