package handlers

import (
	"arunika_backend/models"
	"arunika_backend/services"
	"bytes"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type AuthHandler struct {
	service *services.AuthService
}

func NewAuthHandler(s *services.AuthService) *AuthHandler {
	return &AuthHandler{service: s}
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type SignUpRequest struct {
	Name         string  `json:"name" binding:"required"`
	PhoneNumber  string  `json:"phone_number" binding:"required"`
	EmailAddress string  `json:"email_address" binding:"required"`
	Address      string  `json:"address" binding:"required"`
	City         string  `json:"city" binding:"required"`
	Password     string  `json:"password" binding:"required"`
	Child        []Child `json:"child" binding:"required"`
}

type Child struct {
	Name      string `json:"name" binding:"required"`
	Gender    string `json:"gender" binding:"required"`
	BirthDate string `json:"date_of_birth" binding:"required"`
}

type SignUpResponse struct {
	Name         string  `json:"name" binding:"required"`
	PhoneNumber  string  `json:"phone_number" binding:"required"`
	EmailAddress string  `json:"email" binding:"required"`
	Address      string  `json:"address" binding:"required"`
	City         string  `json:"city" binding:"required"`
	Child        []Child `json:"child" binding:"required"`
	Token        string  `json:"token" binding:"required"`
	RefreshToken string  `json:"refresh_token" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	user, err := h.service.ValidateCredentials(req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	token, refreshToken, err := h.service.GenerateJwtToken(user.ID.String(), user.EmailAddress)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  token,
		"refresh_token": refreshToken,
		"user_id":       user.ID,
	})
}

func (h *AuthHandler) SignUp(c *gin.Context) {
	var req SignUpRequest
	body, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	layout := "2006-01-02T15:04:05.000"
	children := make([]models.Children, len(req.Child))
	for i, child := range req.Child {
		dob, err := time.Parse(layout, child.BirthDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid birth_date format",
			})
			return
		}
		children[i] = models.Children{
			Name:        child.Name,
			Gender:      child.Gender,
			DateOfBirth: dob,
		}
	}
	parent := models.Parent{
		Name:         req.Name,
		PhoneNumber:  req.PhoneNumber,
		EmailAddress: req.EmailAddress,
		Password:     string(hashedPassword),
		Address:      req.Address,
		City:         req.City,
		Children:     children,
	}

	user, err := h.service.Signup(parent)
	if err != nil {
		slog.Error("signup failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	slog.Info("signup success", "user_id", user.ID)
	responseChildren := make([]Child, len(children))
	for i, child := range children {
		responseChildren[i] = Child{
			Name:      child.Name,
			Gender:    child.Gender,
			BirthDate: "2006-01-02T15:04:05.000",
		}
	}
	token, refreshToken, err := h.service.GenerateJwtToken(user.ID.String(), user.EmailAddress)
	response := SignUpResponse{
		Name:         user.Name,
		PhoneNumber:  user.PhoneNumber,
		EmailAddress: user.EmailAddress,
		Address:      user.Address,
		City:         user.City,
		Child:        responseChildren,
		Token:        token,
		RefreshToken: refreshToken,
	}
	c.JSON(http.StatusCreated, gin.H{"data": response})
}

// CheckAvailability handles GET /auth/check-availability?email=&phone=,
// letting the signup flow warn the user before they fill in child data
// instead of only failing at final submit.
func (h *AuthHandler) CheckAvailability(c *gin.Context) {
	email := c.Query("email")
	phone := c.Query("phone")

	emailTaken, phoneTaken, err := h.service.CheckAvailability(email, phone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"email_taken": emailTaken,
		"phone_taken": phoneTaken,
	})
}

func (h *AuthHandler) SendOtp(c *gin.Context) {
	var req models.Parent
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	_, err := h.service.SendOtp(req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "OTP sent successfully to your email address.",
		"expires_in": 300})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	tokenUserID, _ := c.Get("userID")
	refreshTokenParam, _ := c.Get("refresh_token")
	email, _ := c.Get("email")
	if refreshTokenParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing refresh token"})
		return
	}

	userID, err := h.service.ValidateRefreshToken(tokenUserID.(string), refreshTokenParam.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
		return
	}

	accessToken, refreshToken, err := h.service.GenerateJwtToken(userID, email.(string))
	if err != nil {
		slog.Error("token generation failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate new tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	refreshToken, _ := c.Get("refresh_token")
	jti, _ := c.Get("jti")
	exp, _ := c.Get("exp")
	expTime, ok := exp.(time.Time)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expiration format"})
		return
	}
	err := h.service.Logout(c, refreshToken.(string), jti.(string), expTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email"})
		return
	}

	// Always respond the same way whether or not the email is registered, or
	// whether the send actually succeeded — surfacing the difference would
	// let this endpoint enumerate registered accounts. Real failures are
	// logged server-side instead of being shown to the caller.
	if err := h.service.ForgotPassword(req.Email); err != nil {
		slog.Error("forgot password request failed", "error", err)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password reset link sent to " + req.Email})
}

// ResetPasswordPage serves the static web page the emailed reset link points
// at. The token itself is parsed client-side from the URL query string; this
// handler only serves the HTML/JS shell — the actual reset happens via the
// POST endpoint below, registered on the same path.
func (h *AuthHandler) ResetPasswordPage(c *gin.Context) {
	c.File("templates/reset_password.html")
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	token := c.Query("token") // get token from query parameter
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing token"})
		return
	}

	var req struct {
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 8 characters"})
		return
	}

	if err := h.service.ResetPassword(token, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successful"})
}
