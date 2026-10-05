// Package storage stores uploaded media files and builds their public URLs.
//
// Two drivers: "r2" writes to S3-compatible object storage (Cloudflare R2)
// served from a custom domain; "local" writes to a directory that the API
// serves under /media/ — for development and the e2e stack only.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	DriverLocal = "local"
	DriverR2    = "r2"
)

// Store puts objects and turns keys into public URLs.
type Store interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	URL(key string) string
}

// Config is the MEDIA_* environment.
type Config struct {
	Driver        string
	PublicBaseURL string
	LocalDir      string
	S3Endpoint    string
	S3Bucket      string
	S3AccessKey   string
	S3SecretKey   string
	Production    bool
}

// ConfigFromEnv reads MEDIA_* (and APP_ENV) with development defaults.
func ConfigFromEnv() Config {
	c := Config{
		Driver:        os.Getenv("MEDIA_DRIVER"),
		PublicBaseURL: os.Getenv("MEDIA_PUBLIC_BASE_URL"),
		LocalDir:      os.Getenv("MEDIA_LOCAL_DIR"),
		S3Endpoint:    os.Getenv("MEDIA_S3_ENDPOINT"),
		S3Bucket:      os.Getenv("MEDIA_S3_BUCKET"),
		S3AccessKey:   os.Getenv("MEDIA_S3_ACCESS_KEY_ID"),
		S3SecretKey:   os.Getenv("MEDIA_S3_SECRET_ACCESS_KEY"),
		Production:    os.Getenv("APP_ENV") == "production",
	}
	if c.Driver == "" {
		c.Driver = DriverLocal
	}
	if c.Driver == DriverLocal {
		if c.LocalDir == "" {
			c.LocalDir = filepath.Join(os.TempDir(), "arunika-media")
		}
		// A relative URL: each client resolves it against the API host it
		// called, so a phone on ngrok and the backoffice on localhost both
		// reach the files.
		if c.PublicBaseURL == "" {
			c.PublicBaseURL = "/media/"
		}
	}
	if c.PublicBaseURL != "" && !strings.HasSuffix(c.PublicBaseURL, "/") {
		c.PublicBaseURL += "/"
	}
	return c
}

// Validate rejects configurations that would ship broken media: production
// must use R2 behind a custom domain, because r2.dev is blocked by Internet
// Positif in Indonesia.
func (c Config) Validate() error {
	switch c.Driver {
	case DriverLocal:
		if c.Production {
			return errors.New("MEDIA_DRIVER must be r2 in production")
		}
	case DriverR2:
		for name, v := range map[string]string{
			"MEDIA_S3_ENDPOINT": c.S3Endpoint, "MEDIA_S3_BUCKET": c.S3Bucket,
			"MEDIA_S3_ACCESS_KEY_ID": c.S3AccessKey, "MEDIA_S3_SECRET_ACCESS_KEY": c.S3SecretKey,
			"MEDIA_PUBLIC_BASE_URL": c.PublicBaseURL,
		} {
			if v == "" {
				return fmt.Errorf("%s is required when MEDIA_DRIVER=r2", name)
			}
		}
	default:
		return fmt.Errorf("unknown MEDIA_DRIVER %q", c.Driver)
	}
	if c.Driver == DriverLocal && c.PublicBaseURL == "/media/" {
		return nil
	}
	u, err := url.Parse(c.PublicBaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("MEDIA_PUBLIC_BASE_URL %q must be an absolute http(s) URL", c.PublicBaseURL)
	}
	if c.Production && (u.Scheme != "https" || strings.HasSuffix(u.Hostname(), "r2.dev")) {
		return fmt.Errorf("MEDIA_PUBLIC_BASE_URL %q must be an https custom domain in production (r2.dev is blocked in Indonesia)", c.PublicBaseURL)
	}
	return nil
}

// New builds the configured store.
func New(c Config) (Store, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if c.Driver == DriverLocal {
		return &Local{Dir: c.LocalDir, BaseURL: c.PublicBaseURL}, nil
	}
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(c.S3Endpoint),
		Region:       "auto",
		Credentials:  credentials.NewStaticCredentialsProvider(c.S3AccessKey, c.S3SecretKey, ""),
		UsePathStyle: true,
	})
	return &R2{client: client, bucket: c.S3Bucket, baseURL: c.PublicBaseURL}, nil
}

// MustNew is New for startup wiring, where a bad configuration is fatal
// (main.go validates the environment before this runs).
func MustNew(c Config) Store {
	s, err := New(c)
	if err != nil {
		panic("media storage: " + err.Error())
	}
	return s
}

// Local writes objects to a directory.
type Local struct {
	Dir     string
	BaseURL string
}

func (l *Local) Put(_ context.Context, key, _ string, data []byte) error {
	path := filepath.Join(l.Dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (l *Local) URL(key string) string { return l.BaseURL + key }

// R2 writes objects to an S3-compatible bucket.
type R2 struct {
	client  *s3.Client
	bucket  string
	baseURL string
}

func (r *R2) Put(ctx context.Context, key, contentType string, data []byte) error {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
		// Keys are content hashes, so an object never changes.
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	})
	return err
}

func (r *R2) URL(key string) string { return r.baseURL + key }
