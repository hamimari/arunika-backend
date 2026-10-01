package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A syntactically valid service account; the key is never used because no
// token is requested.
const testServiceAccountJSON = `{"type":"service_account","project_id":"arunika-test","private_key_id":"k","private_key":"-----BEGIN PRIVATE KEY-----\nMIIB\n-----END PRIVATE KEY-----\n","client_email":"fcm@arunika-test.iam.gserviceaccount.com","client_id":"1","token_uri":"https://oauth2.googleapis.com/token"}`

func TestFCMCredentials_AcceptsRawJSON(t *testing.T) {
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", testServiceAccountJSON)
	_, projectID, ok, err := NewNotificationService(nil).fcmCredentials()
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "arunika-test", projectID)
}

func TestFCMCredentials_AcceptsFilePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "firebase-service-account.json")
	require.NoError(t, os.WriteFile(path, []byte(testServiceAccountJSON), 0o600))
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", path)

	_, projectID, ok, err := NewNotificationService(nil).fcmCredentials()
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "arunika-test", projectID)
}

func TestFCMCredentials_MissingFileExplainsTheProblem(t *testing.T) {
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", "/path/to/firebase-service-account.json")
	_, _, ok, err := NewNotificationService(nil).fcmCredentials()
	require.Error(t, err)
	assert.False(t, ok)
	assert.Contains(t, err.Error(), "neither JSON nor a readable file path")
	assert.Contains(t, err.Error(), "/path/to/firebase-service-account.json")
}

func TestFCMCredentials_UnsetMeansNotConfigured(t *testing.T) {
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", "")
	_, _, ok, err := NewNotificationService(nil).fcmCredentials()
	require.NoError(t, err)
	assert.False(t, ok)
}
