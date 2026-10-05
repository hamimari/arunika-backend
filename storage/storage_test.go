package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func r2Config() Config {
	return Config{
		Driver: DriverR2, PublicBaseURL: "https://media.haloarunika.com/",
		S3Endpoint: "https://acc.r2.cloudflarestorage.com", S3Bucket: "media",
		S3AccessKey: "k", S3SecretKey: "s", Production: true,
	}
}

func TestValidate(t *testing.T) {
	assert.NoError(t, r2Config().Validate())

	local := Config{Driver: DriverLocal, PublicBaseURL: "http://localhost:8080/media/"}
	assert.NoError(t, local.Validate())
	local.Production = true
	assert.Error(t, local.Validate(), "local driver is refused in production")

	blocked := r2Config()
	blocked.PublicBaseURL = "https://pub-123.r2.dev/"
	assert.Error(t, blocked.Validate(), "r2.dev is blocked in Indonesia")

	missing := r2Config()
	missing.S3Bucket = ""
	assert.Error(t, missing.Validate())

	assert.NoError(t, Config{Driver: DriverLocal, PublicBaseURL: "/media/"}.Validate())

	relative := r2Config()
	relative.PublicBaseURL = "/media/"
	assert.Error(t, relative.Validate())

	assert.Error(t, Config{Driver: "ftp", PublicBaseURL: "http://x/"}.Validate())
}

func TestConfigFromEnvDefaultsToLocal(t *testing.T) {
	for _, k := range []string{"MEDIA_DRIVER", "MEDIA_PUBLIC_BASE_URL", "MEDIA_LOCAL_DIR", "APP_ENV"} {
		t.Setenv(k, "")
	}
	c := ConfigFromEnv()
	assert.Equal(t, DriverLocal, c.Driver)
	assert.Equal(t, "/media/", c.PublicBaseURL, "relative, resolved by each client against its API host")
	assert.NoError(t, c.Validate())

	t.Setenv("MEDIA_PUBLIC_BASE_URL", "https://media.example.com")
	assert.Equal(t, "https://media.example.com/", ConfigFromEnv().PublicBaseURL, "a trailing slash is added")
}

func TestLocalPutAndURL(t *testing.T) {
	dir := t.TempDir()
	s, err := New(Config{Driver: DriverLocal, LocalDir: dir, PublicBaseURL: "http://localhost/media/"})
	require.NoError(t, err)
	require.NoError(t, s.Put(context.Background(), "huruf/abc.png", "image/png", []byte("x")))
	got, err := os.ReadFile(filepath.Join(dir, "huruf", "abc.png"))
	require.NoError(t, err)
	assert.Equal(t, "x", string(got))
	assert.Equal(t, "http://localhost/media/huruf/abc.png", s.URL("huruf/abc.png"))
}

func TestMustNewPanicsOnBadConfig(t *testing.T) {
	assert.Panics(t, func() { MustNew(Config{Driver: "ftp"}) })
	r2 := MustNew(r2Config())
	assert.Equal(t, "https://media.haloarunika.com/huruf/x.png", r2.URL("huruf/x.png"))
}
