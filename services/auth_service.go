package services

import (
	"arunika_backend/models"
	"arunika_backend/utils"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"html/template"
	"log/slog"
	"os"
	"time"
)

type AuthService struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewAuthService(db *gorm.DB, redis *redis.Client) *AuthService {
	return &AuthService{db: db, redis: redis}
}

// SessionLifetime is how long a session survives without being used. It is a
// sliding window: every refresh pushes the expiry back to a full week from
// now, so someone who opens the app at least once a week stays signed in,
// while an abandoned session lapses a week after its last use.
const SessionLifetime = 7 * 24 * time.Hour

// ErrRefreshTokenInvalid means the session is genuinely over — the token is
// unknown, already rotated away, or lapsed — and the user must sign in again.
// Any other error from RefreshSession is a server-side failure, where the
// client should retry rather than drop the session.
var ErrRefreshTokenInvalid = errors.New("invalid or expired refresh token")

// issueTokens mints an access/refresh token pair and persists the refresh
// token with a fresh expiry.
func (s *AuthService) issueTokens(userId, email string) (string, string, error) {
	token, refreshToken, err := utils.GenerateJWT(userId, email)
	if err != nil {
		return "", "", err
	}

	refreshTokenEntity := models.RefreshToken{
		UserId:    userId,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(SessionLifetime),
	}
	if err := s.db.Create(&refreshTokenEntity).Error; err != nil {
		return "", "", err
	}
	return token, refreshToken, nil
}

func (s *AuthService) GenerateJwtToken(userId, email string) (string, string, error) {
	token, refreshToken, err := s.issueTokens(userId, email)
	if err != nil {
		return "", "", err
	}

	// Record user session for DAU tracking (best-effort, non-blocking).
	// Only real sign-ins count — refreshes would inflate the numbers.
	if uid, parseErr := uuid.Parse(userId); parseErr == nil {
		session := models.UserSession{UserID: uid}
		go s.db.Create(&session)
	}

	return token, refreshToken, nil
}

// RefreshSession exchanges a valid refresh token for a new token pair. The
// refresh token is the only credential needed: the access token it was issued
// alongside lives 15 minutes, so by the time a user reopens the app it has
// almost always expired.
//
// The old refresh token is rotated out, so each one is usable exactly once and
// the table doesn't grow without bound.
func (s *AuthService) RefreshSession(refreshToken string) (string, string, error) {
	stored, err := models.FindRefreshToken(s.db, refreshToken)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", ErrRefreshTokenInvalid
		}
		return "", "", err
	}
	if time.Now().After(stored.ExpiresAt) {
		// Lapsed session: drop the row so it can't be probed again.
		_ = models.DeleteByToken(s.db, refreshToken)
		return "", "", ErrRefreshTokenInvalid
	}

	user, err := models.FindUserById(s.db, stored.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", ErrRefreshTokenInvalid
		}
		return "", "", err
	}
	if user.IsDeleted {
		return "", "", ErrRefreshTokenInvalid
	}

	accessToken, newRefreshToken, err := s.issueTokens(stored.UserId, user.EmailAddress)
	if err != nil {
		return "", "", err
	}
	// Only revoke the old token once its replacement is safely stored, so a
	// failure here can never leave the user with no valid refresh token.
	if delErr := models.DeleteByToken(s.db, refreshToken); delErr != nil {
		slog.Warn("refresh: failed to delete rotated token", "error", delErr)
	}

	return accessToken, newRefreshToken, nil
}

func (s *AuthService) Signup(request models.Parent) (models.Parent, error) {
	user, _ := models.FindUserByEmail(s.db, request.EmailAddress)
	if user != nil {
		return models.Parent{}, errors.New("email address already taken")
	}

	if existing, _ := models.FindUserByPhoneNumber(s.db, request.PhoneNumber); existing != nil {
		return models.Parent{}, errors.New("phone number already taken")
	}

	if err := s.db.Create(&request).Error; err != nil {
		return models.Parent{}, err
	}

	return request, nil
}

// CheckAvailability reports whether email/phone are already taken by an
// existing account, so the client can surface this before the user fills in
// child data (rather than only failing at final submit).
func (s *AuthService) CheckAvailability(email, phone string) (emailTaken bool, phoneTaken bool, err error) {
	if email != "" {
		user, findErr := models.FindUserByEmail(s.db, email)
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return false, false, findErr
		}
		emailTaken = user != nil
	}
	if phone != "" {
		user, findErr := models.FindUserByPhoneNumber(s.db, phone)
		if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return false, false, findErr
		}
		phoneTaken = user != nil
	}
	return emailTaken, phoneTaken, nil
}

func (s *AuthService) ValidateCredentials(email string, password string) (*models.Parent, error) {
	user, _ := models.FindUserByEmail(s.db, email)
	if user == nil || user.IsDeleted || !models.CheckPassword(user.Password, password) {
		return nil, errors.New("invalid credential")
	}
	return user, nil
}

func (s *AuthService) Logout(ctx *gin.Context, token string, jti string, exp time.Time) error {
	err := models.DeleteByToken(s.db, token)
	if err != nil {
		return errors.New("token not found")
	}

	return s.RevokeToken(ctx, jti, exp)
}

// RevokeToken blacklists jti until exp, without requiring a specific
// refresh token row to exist — used by account deletion, which already
// removes every refresh token for the user in bulk.
func (s *AuthService) RevokeToken(ctx *gin.Context, jti string, exp time.Time) error {
	ttl := time.Until(exp)
	if ttl <= 0 {
		ttl = time.Minute * 15
	}
	return s.redis.Set(ctx, "blacklist:"+jti, "revoked", ttl).Err()
}

// ForgotPassword always returns nil for an unknown email — the caller
// (AuthHandler.ForgotPassword) must respond identically whether or not the
// account exists, otherwise this endpoint becomes an account-enumeration
// oracle. Real failures (DB/email) are returned so they can be logged
// server-side, but are never turned into a different client-visible outcome.
func (s *AuthService) ForgotPassword(email string) error {
	var user models.Parent
	if err := s.db.Where("email_address = ?", email).First(&user).Error; err != nil {
		return nil
	}

	// An unverified address is one nobody has proven they control, so mailing
	// a password-reset link to it would hand account recovery to whoever
	// happens to own that mailbox — including for an account they never
	// created. This is the only thing email verification gates.
	//
	// It returns nil, exactly as the unknown-email branch above does: the
	// caller must not be able to tell "no such account" from "not verified"
	// from "sent". Returning an error here instead would turn this endpoint
	// into an oracle for which addresses are verified.
	if !user.EmailVerified {
		slog.Info("password reset withheld: email not verified", "user_id", user.ID)
		return nil
	}

	// Invalidate any previously issued, still-valid reset tokens for this
	// user so only the newest emailed link works.
	if err := s.db.Where("user_id = ?", user.ID).Delete(&models.PasswordResetToken{}).Error; err != nil {
		return err
	}

	resetToken := uuid.NewString()
	reset := models.PasswordResetToken{
		UserID: user.ID,
		// Store only a hash — see PasswordResetToken.Token doc comment.
		Token:     utils.HashResetToken(resetToken),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	if err := s.db.Create(&reset).Error; err != nil {
		return err
	}

	resetLink := fmt.Sprintf("%s/reset-password?token=%s", os.Getenv("APP_DOMAIN"), resetToken)
	subject := "Password Reset Request"
	body := fmt.Sprintf(
		"<p>Hi %s,</p><p>Click the link below to reset your password:</p><p><a href=\"%s\">%s</a></p><p>This link will expire in 15 minutes.</p><p>Best,<br>Arunika Team</p>",
		template.HTMLEscapeString(user.Name), resetLink, resetLink,
	)

	// Goes through the same SMTP path as OTP/campaign email (gomail, reads
	// SMTP_PASS) rather than the old net/smtp-based utils.SendEmail, which
	// read a "SMTP_PASSWORD" variable that was never set anywhere in this
	// codebase — every password-reset email silently failed to send.
	return SendGenericEmail(user.EmailAddress, subject, body)
}

func (s *AuthService) ResetPassword(token, newPassword string) error {
	reset, err := s.VerifyResetToken(token)
	if err != nil {
		return err
	}

	var user models.Parent
	if err := s.db.First(&user, reset.UserID).Error; err != nil {
		return errors.New("user not found")
	}

	bytes, err := bcrypt.GenerateFromPassword([]byte(newPassword), 14)
	if err != nil {
		return err
	}
	hashed := string(bytes)

	user.Password = hashed
	if err := s.db.Save(&user).Error; err != nil {
		return err
	}

	s.db.Delete(&reset)

	// Revoke existing sessions: a leaked/compromised refresh token must stop
	// working the moment the password changes. Already-issued access tokens
	// remain valid until their own (short, 15-minute) expiry — revoking
	// those too would mean checking every authenticated request against a
	// per-user "password changed at" timestamp, a change to the hot
	// JWTAuthMiddleware path affecting every request, not just this flow.
	if err := models.DeleteAllRefreshTokensByUserId(s.db, reset.UserID.String()); err != nil {
		slog.Error("failed to revoke refresh tokens after password reset", "user_id", reset.UserID, "error", err)
	}

	return nil
}

func (s *AuthService) VerifyResetToken(token string) (*models.PasswordResetToken, error) {
	var reset models.PasswordResetToken
	if err := s.db.Where("token = ?", utils.HashResetToken(token)).First(&reset).Error; err != nil {
		return nil, errors.New("invalid token")
	}

	if time.Now().After(reset.ExpiresAt) {
		return nil, errors.New("token expired")
	}
	return &reset, nil
}

// ─── Email verification ───────────────────────────────────────────────────────

// ErrEmailAlreadyVerified is returned when there is nothing to do — the
// caller should treat it as success, not failure.
var ErrEmailAlreadyVerified = errors.New("email is already verified")

// ErrVerificationTokenInvalid covers unknown, expired and already-consumed
// tokens alike. They are deliberately indistinguishable to the caller: the
// user-facing outcome ("this link no longer works, request another") is the
// same, and separating them would let someone probe which tokens once
// existed.
var ErrVerificationTokenInvalid = errors.New("verification link is invalid or has expired")

// IssueEmailVerificationToken creates a fresh verification token for a user
// and returns the raw value to be emailed. Only its hash is stored, and any
// previously issued token for that user is deleted, so exactly one link is
// live at a time.
//
// The raw token is a UUIDv4 from github.com/google/uuid, which draws from
// crypto/rand — the same source the password-reset flow uses.
func (s *AuthService) IssueEmailVerificationToken(userID uuid.UUID) (string, error) {
	rawToken := uuid.NewString()
	expiresAt := time.Now().Add(models.EmailVerificationTokenLifetime)

	if err := models.CreateEmailVerificationToken(
		s.db, userID, utils.HashToken(rawToken), expiresAt,
	); err != nil {
		return "", err
	}
	return rawToken, nil
}

// ConsumeEmailVerificationToken marks the account verified and destroys the
// token, so a link works exactly once.
func (s *AuthService) ConsumeEmailVerificationToken(rawToken string) error {
	_, err := s.ConsumeEmailVerificationTokenForPage(rawToken)
	return err
}

// ConsumeEmailVerificationTokenForPage is ConsumeEmailVerificationToken plus
// the account holder's name, so the confirmation page can greet them.
//
// It returns ErrEmailAlreadyVerified (with the name) when a still-valid token
// belongs to an account that has since been verified some other way — that is
// a success for the user, not a failure. A token that has already been
// consumed cannot be told apart from one that never existed, because consumption
// deletes it; both yield ErrVerificationTokenInvalid, and the page offers a
// resend path for exactly that case.
func (s *AuthService) ConsumeEmailVerificationTokenForPage(rawToken string) (string, error) {
	if rawToken == "" {
		return "", ErrVerificationTokenInvalid
	}

	token, err := models.FindEmailVerificationToken(s.db, utils.HashToken(rawToken))
	if err != nil {
		return "", ErrVerificationTokenInvalid
	}

	var user models.Parent
	if err := s.db.Where("id = ?", token.UserID).First(&user).Error; err != nil {
		return "", ErrVerificationTokenInvalid
	}

	if user.EmailVerified {
		_ = models.DeleteEmailVerificationTokensForUser(s.db, token.UserID)
		return user.Name, ErrEmailAlreadyVerified
	}

	if time.Now().After(token.ExpiresAt) {
		// Clear the dead token so it cannot linger and so a resend starts
		// from a clean slate.
		_ = models.DeleteEmailVerificationTokensForUser(s.db, token.UserID)
		return "", ErrVerificationTokenInvalid
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := models.MarkEmailVerified(tx, token.UserID); err != nil {
			return err
		}
		return models.DeleteEmailVerificationTokensForUser(tx, token.UserID)
	}); err != nil {
		return "", err
	}
	return user.Name, nil
}

// IsEmailVerified reports whether a user has proven control of their address.
func (s *AuthService) IsEmailVerified(userID uuid.UUID) (bool, error) {
	var user models.Parent
	if err := s.db.Select("email_verified").Where("id = ?", userID).First(&user).Error; err != nil {
		return false, err
	}
	return user.EmailVerified, nil
}

// ResendVerificationEmail issues a new token and emails it. It reports
// ErrEmailAlreadyVerified when there is nothing to do, so the caller can
// respond successfully without issuing a pointless token.
func (s *AuthService) ResendVerificationEmail(userID uuid.UUID) error {
	var user models.Parent
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return err
	}
	if user.EmailVerified {
		return ErrEmailAlreadyVerified
	}

	rawToken, err := s.IssueEmailVerificationToken(user.ID)
	if err != nil {
		return err
	}
	return SendVerificationEmail(user.EmailAddress, user.Name, rawToken)
}
