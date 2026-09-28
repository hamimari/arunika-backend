package handlers

import (
	"arunika_backend/models"
	"arunika_backend/services"
	"bytes"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// RefreshTokenRequest carries the refresh token in the body — the endpoint
// takes no Authorization header, since the access token has normally expired
// by the time a refresh is needed.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

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
	// ID lets the app store the new user's id right away, as it does after
	// login — without it the app can't fetch the fresh profile until the
	// user signs in again.
	ID           string  `json:"id"`
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

// sendVerificationEmailAsync issues a verification token and emails it
// without blocking the response.
//
// Registration must never fail because mail delivery did. SMTP is a
// third-party dependency that fails independently of Arunika, and this
// codebase has already shipped a bug where every password-reset email failed
// silently because the sending path read an env var set nowhere. Putting
// that class of failure on the critical path of account creation would turn
// a mail outage into a signup outage — so the error is logged with the
// user's id and goes no further. The user can resend from the app.
func (h *AuthHandler) sendVerificationEmailAsync(userID uuid.UUID, email, name string) {
	go func() {
		rawToken, err := h.service.IssueEmailVerificationToken(userID)
		if err != nil {
			slog.Error("could not issue email verification token",
				"user_id", userID, "error", err)
			return
		}
		if err := services.SendVerificationEmail(email, name, rawToken); err != nil {
			slog.Error("could not send verification email",
				"user_id", userID, "error", err)
		}
	}()
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
	h.sendVerificationEmailAsync(user.ID, user.EmailAddress, user.Name)
	responseChildren := make([]Child, len(children))
	for i, child := range children {
		responseChildren[i] = Child{
			Name:      child.Name,
			Gender:    child.Gender,
			BirthDate: "2006-01-02T15:04:05.000",
		}
	}
	token, refreshToken, err := h.service.GenerateJwtToken(user.ID.String(), user.EmailAddress)
	if err != nil {
		// The account is created and committed by this point, so this is not a
		// signup failure — only the session could not be issued. Say that
		// explicitly: the user's credentials are valid and signing in will
		// work, so "please sign in" is the one actionable instruction.
		//
		// Previously this error was discarded and the handler returned 201
		// with empty token/refresh_token. The app refuses to proceed on an
		// empty token (signup_bloc.dart), so the user saw a generic failure
		// for an account that had in fact been created — and retrying hit
		// "email address already taken". Nothing reached the logs either.
		slog.Error("signup succeeded but token generation failed",
			"user_id", user.ID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Account created, but the session could not be started. Please sign in.",
		})
		return
	}
	response := SignUpResponse{
		ID:           user.ID.String(),
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

// RefreshToken exchanges a refresh token for a new token pair. It is
// deliberately not behind JWTAuthMiddleware: the whole point of refreshing is
// that the access token has expired, so requiring a valid one would sign
// people out after 15 minutes of not using the app.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.RefreshToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing refresh token"})
		return
	}

	accessToken, refreshToken, err := h.service.RefreshSession(strings.TrimSpace(req.RefreshToken))
	if err != nil {
		if errors.Is(err, services.ErrRefreshTokenInvalid) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
			return
		}
		// A server-side failure must not read as "your session is over" —
		// 500 tells the client to retry instead of signing the user out.
		slog.Error("token refresh failed", "error", err)
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

// ─── Email verification ───────────────────────────────────────────────────────

// verificationPageData fills templates/email_verified.html. Status is one of
// "verified", "already" or "invalid". The token is deliberately absent — it
// must never be rendered into the page.
type verificationPageData struct {
	Status string
	Name   string
}

// VerifyEmail handles GET /auth/verify-email?token=.
//
// It renders an HTML page rather than returning JSON because the link is
// opened from a mail client, potentially on a different device from the app.
// A server-rendered page works everywhere with no deep-link plumbing.
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	tmpl, err := template.ParseFiles("templates/email_verified.html")
	if err != nil {
		slog.Error("could not parse verification page template", "error", err)
		c.String(http.StatusInternalServerError, "Verification page unavailable")
		return
	}

	render := func(status, name string) {
		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		// Belt and braces alongside the page's own meta referrer tag.
		c.Header("Referrer-Policy", "no-referrer")
		if err := tmpl.Execute(c.Writer, verificationPageData{Status: status, Name: name}); err != nil {
			slog.Error("could not render verification page", "error", err)
		}
	}

	token := c.Query("token")
	name, err := h.service.ConsumeEmailVerificationTokenForPage(token)
	switch {
	case err == nil:
		render("verified", name)
	case errors.Is(err, services.ErrEmailAlreadyVerified):
		render("already", name)
	default:
		// Unknown, expired and already-consumed tokens are deliberately
		// indistinguishable — the user-facing remedy is the same, and
		// separating them would let someone probe which tokens once existed.
		render("invalid", "")
	}
}

// ResendVerification handles POST /auth/resend-verification for the signed-in
// user. Rate limiting is applied by middleware at the route.
func (h *AuthHandler) ResendVerification(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID, err := uuid.Parse(userIDVal.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	err = h.service.ResendVerificationEmail(userID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"message": "Verification email sent."})
	case errors.Is(err, services.ErrEmailAlreadyVerified):
		// Nothing to do is a success, not an error — the caller's goal
		// (a verified address) is already met.
		c.JSON(http.StatusOK, gin.H{"message": "Email is already verified."})
	default:
		slog.Error("could not resend verification email", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Could not send the verification email. Please try again later.",
		})
	}
}
