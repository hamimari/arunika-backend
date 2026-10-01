package api_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
	"arunika_backend/tests/fixtures"
)

// The full authentication lifecycle, exercised over HTTP through the real
// router and middleware chain against a real database.
//
// Flows 6 (token refresh) and 7 (logout revocation) from the strategy's
// critical-journey list live here rather than in the cross-system E2E suite:
// they need no UI and no compose stack, so proving them at this tier is
// faster and more reliable for identical confidence.

func TestAuth_RegisterThenAccessProtectedResource(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	account := env.Register(t)

	require.NotEmpty(t, account.Token)
	require.NotEmpty(t, account.RefreshToken)

	// The issued token actually opens a protected route.
	res := env.GET("/fairy-tales/history", account.Token)
	assert.NotEqual(t, http.StatusUnauthorized, res.Code,
		"a freshly issued token must authorise protected requests")
}

func TestAuth_NewAccountStartsUnverified(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	account := env.Register(t)

	var user models.Parent
	require.NoError(t, env.DB.Where("email_address = ?", account.Email).First(&user).Error)
	assert.False(t, user.EmailVerified,
		"registration must not mark an address verified before anyone proves control of it")
}

func TestAuth_Login_ValidCredentials_IssuesTokens(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	account := env.Register(t)

	res := env.Login(t, account.Email, account.Password)

	require.Equal(t, http.StatusOK, res.Code)
	body := res.JSON()
	assert.NotEmpty(t, body["access_token"])
	assert.NotEmpty(t, body["refresh_token"])
	assert.NotEmpty(t, body["user_id"])
}

func TestAuth_Login_InvalidCredentials_AreRejectedIndistinguishably(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	account := env.Register(t)

	wrongPassword := env.Login(t, account.Email, "not-the-password")
	unknownEmail := env.Login(t, "nobody@example.test", "secret123")

	assert.Equal(t, http.StatusUnauthorized, wrongPassword.Code)
	assert.Equal(t, http.StatusUnauthorized, unknownEmail.Code)
	// Telling these apart would let an attacker enumerate registered addresses.
	assert.JSONEq(t, string(wrongPassword.Body), string(unknownEmail.Body),
		"a wrong password must be indistinguishable from an unknown account")
}

func TestAuth_ProtectedRoute_RejectsMissingAndMalformedTokens(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	for name, token := range map[string]string{
		"missing header": "",
		"malformed jwt":  "not.a.jwt",
		"empty bearer":   " ",
	} {
		t.Run(name, func(t *testing.T) {
			res := env.GET("/fairy-tales/history", token)
			assert.Equal(t, http.StatusUnauthorized, res.Code)
		})
	}
}

// Flow 6: the access token lives 15 minutes, so the refresh token is the
// credential that keeps a returning user signed in.
func TestAuth_RefreshToken_IssuesAWorkingNewSession(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	account := env.Register(t)

	res := env.POST("/auth/refresh-token", "", map[string]string{
		"refresh_token": account.RefreshToken,
	})

	require.Equal(t, http.StatusOK, res.Code, "body: %s", string(res.Body))
	body := res.JSON()
	newAccess, _ := body["access_token"].(string)
	require.NotEmpty(t, newAccess)

	// The replacement token must actually work.
	assert.NotEqual(t, http.StatusUnauthorized,
		env.GET("/fairy-tales/history", newAccess).Code)
}

// Refresh tokens rotate: each one is usable exactly once, so a stolen token
// cannot be replayed after the legitimate client has used it.
func TestAuth_RefreshToken_IsSingleUse(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	account := env.Register(t)

	first := env.POST("/auth/refresh-token", "", map[string]string{
		"refresh_token": account.RefreshToken,
	})
	require.Equal(t, http.StatusOK, first.Code)

	replay := env.POST("/auth/refresh-token", "", map[string]string{
		"refresh_token": account.RefreshToken,
	})

	assert.Equal(t, http.StatusUnauthorized, replay.Code,
		"a rotated refresh token must not be reusable")
}

func TestAuth_RefreshToken_UnknownOrMissing_IsRejected(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	unknown := env.POST("/auth/refresh-token", "", map[string]string{
		"refresh_token": "00000000-0000-0000-0000-000000000000",
	})
	assert.Equal(t, http.StatusUnauthorized, unknown.Code)

	missing := env.POST("/auth/refresh-token", "", map[string]string{})
	assert.Equal(t, http.StatusBadRequest, missing.Code)
}

// Flow 7: logging out must actually end the session. The access token stays
// cryptographically valid until it expires, so revocation is enforced by the
// Redis blacklist the middleware consults.
func TestAuth_Logout_RevokesTheAccessTokenImmediately(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	account := env.Register(t)

	require.NotEqual(t, http.StatusUnauthorized,
		env.GET("/fairy-tales/history", account.Token).Code, "sanity: signed in")

	logout := env.POST("/auth/logout", account.Token, nil)
	require.Equal(t, http.StatusOK, logout.Code, "body: %s", string(logout.Body))

	after := env.GET("/fairy-tales/history", account.Token)
	assert.Equal(t, http.StatusUnauthorized, after.Code,
		"the same token must stop working the moment the user logs out")
}

// The app stores the returned id right after signup (as it does after
// login) so it can fetch the new user's profile without a second sign-in.
func TestAuth_Signup_ReturnsTheSameUserIDAsLogin(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	account := env.Register(t)
	require.NotEmpty(t, account.ID)

	res := env.Login(t, account.Email, account.Password)
	require.Equal(t, http.StatusOK, res.Code, "login: %s", string(res.Body))
	assert.Equal(t, res.JSON()["user_id"], account.ID)
}

func TestAuth_Signup_DuplicateEmail_IsRejected(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	account := env.Register(t)

	res := env.POST("/auth/signup", "", map[string]interface{}{
		"name":          "Impostor",
		"phone_number":  "081999999999",
		"email_address": account.Email,
		"address":       "Jl. Other",
		"city":          "Bandung",
		"password":      "secret123",
		"child": []map[string]string{
			{"name": "Ani", "gender": "F", "date_of_birth": "2021-03-04T00:00:00.000"},
		},
	})

	assert.Equal(t, http.StatusInternalServerError, res.Code)
	assert.Contains(t, string(res.Body), "already taken")
}

func TestAuth_Signup_InvalidPayload_IsRejectedWithoutCreatingAnAccount(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	res := env.POST("/auth/signup", "", map[string]interface{}{"name": ""})

	require.Equal(t, http.StatusBadRequest, res.Code)

	var count int64
	require.NoError(t, env.DB.Model(&models.Parent{}).Count(&count).Error)
	assert.Zero(t, count, "a rejected signup must leave no account behind")
}

func TestAuth_CheckAvailability_ReportsTakenEmailAndPhone(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)
	account := env.Register(t)

	taken := env.GET("/auth/check-availability?email="+account.Email, "")
	require.Equal(t, http.StatusOK, taken.Code)
	assert.Equal(t, true, taken.JSON()["email_taken"])

	free := env.GET("/auth/check-availability?email=definitely-free@example.test", "")
	require.Equal(t, http.StatusOK, free.Code)
	assert.Equal(t, false, free.JSON()["email_taken"])
}

// Horizontal privilege escalation: one user must never read another's
// profile, even with a perfectly valid token of their own.
func TestAuth_UserCannotReadAnotherUsersProfile(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	victim := fixtures.NewUser(t, env.DB)
	attacker := env.Register(t)

	res := env.GET("/user/"+victim.ID.String(), attacker.Token)

	assert.Equal(t, http.StatusForbidden, res.Code,
		"a user must not be able to fetch another user's profile by id")
}

// The central promise of the email-verification design: registration must not
// depend on mail delivery. SMTP is deliberately left unconfigured in this test
// process, so the verification email genuinely cannot be sent.
//
// This lives here rather than in the handler tests because the dispatch is a
// fire-and-forget goroutine: against an ordered sqlmock it races the main
// request for expectations and intermittently fails the very thing it claims
// to prove. A real database has no such ordering constraint.
func TestAuth_Signup_SucceedsEvenThoughVerificationMailCannotBeSent(t *testing.T) {
	t.Parallel()
	env := NewAPIEnv(t)

	require.Empty(t, os.Getenv("SMTP_HOST"),
		"this test is only meaningful while SMTP is unconfigured")

	account := env.Register(t) // asserts 201 internally

	// A usable session was still issued.
	assert.NotEmpty(t, account.Token)
	assert.NotEmpty(t, account.RefreshToken)
	assert.NotEqual(t, http.StatusUnauthorized,
		env.GET("/fairy-tales/history", account.Token).Code)

	// And the account really exists.
	var count int64
	require.NoError(t, env.DB.Model(&models.Parent{}).
		Where("email_address = ?", account.Email).Count(&count).Error)
	assert.Equal(t, int64(1), count, "a mail failure must not roll back the account")
}
