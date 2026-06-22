package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_EnvOverridesDefaults(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "env-key")
	t.Setenv("UPLOADS_BUCKET", "my-bucket")
	t.Setenv("AWS_REGION", "us-west-2")
	t.Setenv("PORT", "9090")

	withWorkdir(t, t.TempDir())

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.JWTSigningKey != "env-key" {
		t.Errorf("JWTSigningKey = %q", c.JWTSigningKey)
	}
	if c.UploadsBucket != "my-bucket" {
		t.Errorf("UploadsBucket = %q", c.UploadsBucket)
	}
	if c.AWSRegion != "us-west-2" {
		t.Errorf("AWSRegion = %q", c.AWSRegion)
	}
	if c.Port != "9090" {
		t.Errorf("Port = %q", c.Port)
	}
}

func TestLoad_SecretsFileFallback(t *testing.T) {
	dir := t.TempDir()
	withWorkdir(t, dir)
	path := filepath.Join(dir, "secrets.json")
	if err := os.WriteFile(path, []byte(`{"jwt_signing_key":"file-key","uploads_bucket":"file-bucket"}`), 0o600); err != nil {
		t.Fatalf("write secrets.json: %v", err)
	}
	clearEnv(t)

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.JWTSigningKey != "file-key" {
		t.Errorf("JWTSigningKey = %q, want file-key", c.JWTSigningKey)
	}
	if c.UploadsBucket != "file-bucket" {
		t.Errorf("UploadsBucket = %q, want file-bucket", c.UploadsBucket)
	}
}

func TestLoad_EnvBeatsFile(t *testing.T) {
	dir := t.TempDir()
	withWorkdir(t, dir)
	path := filepath.Join(dir, "secrets.json")
	_ = os.WriteFile(path, []byte(`{"jwt_signing_key":"file-key","uploads_bucket":"file-bucket"}`), 0o600)
	clearEnv(t)
	t.Setenv("UPLOADS_BUCKET", "env-wins")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.UploadsBucket != "env-wins" {
		t.Errorf("UploadsBucket = %q, want env-wins", c.UploadsBucket)
	}
	if c.JWTSigningKey != "file-key" {
		t.Errorf("JWTSigningKey = %q, want file-key (only file set)", c.JWTSigningKey)
	}
}

func TestLoad_MissingSigningKey(t *testing.T) {
	withWorkdir(t, t.TempDir())
	clearEnv(t)
	if _, err := Load(); err == nil {
		t.Error("Load must fail when JWT_SIGNING_KEY is unset")
	}
}

func withWorkdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"JWT_SIGNING_KEY", "JWT_ISSUER", "UPLOADS_BUCKET", "AWS_REGION", "PORT", "OKTA_ISSUER", "OKTA_CLIENT_ID", "OKTA_CLIENT_SECRET", "OKTA_REDIRECT_URL", "DEV_AUTH_BYPASS", "SECRETS_FILE"} {
		t.Setenv(k, "")
	}
}
