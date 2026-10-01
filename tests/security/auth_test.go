package security_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"arunika_backend/models"
)

// A route that needs a signed-in user, and nothing else, so a status change
// here can only come from the credential being judged.
const protectedPath = "/fairy-tales/history"

func TestAccessToken_InvalidCredentials_AreAllRejected(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	victim := env.Register(t)

	cases := map[string]string{
		"missing authorization header": "",
		"malformed jwt":                "not.a.jwt",
		"random string":                "Zm9vYmFy",
		"expired token": mint(t, jwt.SigningMethodHS256, []byte(testSecret),
			userClaims(victim.ID, time.Now().Add(-time.Minute))),
		"signed with the wrong key": mint(t, jwt.SigningMethodHS256, []byte("an-attacker-chosen-secret-key-0123456789"),
			userClaims(victim.ID, time.Now().Add(time.Hour))),
		"different HMAC algorithm, wrong key": mint(t, jwt.SigningMethodHS512, []byte("an-attacker-chosen-secret-key-0123456789"),
			userClaims(victim.ID, time.Now().Add(time.Hour))),
		"alg none": mint(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType,
			userClaims(victim.ID, time.Now().Add(time.Hour))),
		"no exp claim": mint(t, jwt.SigningMethodHS256, []byte(testSecret),
			jwt.MapClaims{"sub": victim.ID.String(), "jti": uuid.NewString()}),
		"no jti claim": mint(t, jwt.SigningMethodHS256, []byte(testSecret),
			jwt.MapClaims{"sub": victim.ID.String(), "exp": time.Now().Add(time.Hour).Unix()}),
	}

	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			res := env.GET(protectedPath, token)
			assert.Equal(t, http.StatusUnauthorized, res.Code, "body: %s", string(res.Body))
		})
	}
}

// Sanity for the table above: a correctly signed token for the same user is
// accepted, so the rejections are about the credential and not the route.
func TestAccessToken_CorrectlySignedToken_IsAccepted(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	user := env.Register(t)

	good := mint(t, jwt.SigningMethodHS256, []byte(testSecret),
		userClaims(user.ID, time.Now().Add(time.Hour)))

	assert.Equal(t, http.StatusOK, env.GET(protectedPath, good).Code)
}

func TestAccessToken_ReuseAfterLogout_IsRejected(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	user := env.Register(t)
	require.Equal(t, http.StatusOK, env.GET(protectedPath, user.Token).Code, "sanity: signed in")

	require.Equal(t, http.StatusOK, env.POST("/auth/logout", user.Token, nil).Code)

	assert.Equal(t, http.StatusUnauthorized, env.GET(protectedPath, user.Token).Code,
		"a logged-out access token must be dead even though its signature and expiry are still valid")
	// Revocation lives in Redis; the entry is what the middleware reads.
	assert.NotEmpty(t, env.Redis.Keys(), "logout must leave a blacklist entry behind")
}

func TestRefreshToken_Expired_IsRejectedAndNeverIssuesTokens(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	user := env.Register(t)

	// expires_at is a zoneless TIMESTAMP, so the wall-clock digits Go writes
	// are read back as UTC. Expiring it by two days keeps this test true on a
	// developer machine in any timezone, not only on a UTC server.
	require.NoError(t, env.DB.Model(&models.RefreshToken{}).
		Where("token = ?", user.RefreshToken).
		Update("expires_at", time.Now().Add(-48*time.Hour)).Error)

	res := env.POST("/auth/refresh-token", "", map[string]string{"refresh_token": user.RefreshToken})

	assert.Equal(t, http.StatusUnauthorized, res.Code)
	assert.NotContains(t, string(res.Body), "access_token")
}

func TestRefreshToken_RevokedByLogout_CannotMintANewSession(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	user := env.Register(t)
	require.Equal(t, http.StatusOK, env.POST("/auth/logout", user.Token, nil).Code)

	res := env.POST("/auth/refresh-token", "", map[string]string{"refresh_token": user.RefreshToken})

	assert.Equal(t, http.StatusUnauthorized, res.Code,
		"logging out must also end the refresh token, or the session simply comes back")
}

func TestRefreshToken_Replay_IsRejectedAndOldTokenStaysDead(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	user := env.Register(t)

	first := env.POST("/auth/refresh-token", "", map[string]string{"refresh_token": user.RefreshToken})
	require.Equal(t, http.StatusOK, first.Code)

	for i := 0; i < 3; i++ {
		replay := env.POST("/auth/refresh-token", "", map[string]string{"refresh_token": user.RefreshToken})
		assert.Equal(t, http.StatusUnauthorized, replay.Code, "replay #%d", i+1)
	}
}

func TestRefreshToken_OfADeletedAccount_IsRejected(t *testing.T) {
	t.Parallel()
	env := NewEnv(t)
	user := env.Register(t)
	require.NoError(t, env.DB.Model(&models.Parent{}).Where("id = ?", user.ID).Update("is_deleted", true).Error)

	res := env.POST("/auth/refresh-token", "", map[string]string{"refresh_token": user.RefreshToken})

	assert.Equal(t, http.StatusUnauthorized, res.Code)
}
