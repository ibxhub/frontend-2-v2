package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type Config struct {
	Port            string `json:"port"`
	UploadsBucket   string `json:"uploads_bucket"`
	AWSRegion       string `json:"aws_region"`
	JWTSigningKey   string `json:"jwt_signing_key"`
	JWTIssuer       string `json:"jwt_issuer"`
	OktaIssuer      string `json:"okta_issuer"`
	OktaClientID    string `json:"okta_client_id"`
	OktaClientSecret string `json:"okta_client_secret"`
	OktaRedirectURL string `json:"okta_redirect_url"`
	DevAuthBypass   bool   `json:"dev_auth_bypass"`
}

func Load() (*Config, error) {
	c := &Config{}

	if path := os.Getenv("SECRETS_FILE"); path != "" {
		if err := mergeFromFile(c, path); err != nil {
			return nil, fmt.Errorf("read secrets file: %w", err)
		}
	} else if _, err := os.Stat("secrets.json"); err == nil {
		if err := mergeFromFile(c, "secrets.json"); err != nil {
			return nil, fmt.Errorf("read secrets.json: %w", err)
		}
	}

	overlay(&c.Port, "PORT", "8080")
	overlay(&c.UploadsBucket, "UPLOADS_BUCKET", "")
	overlay(&c.AWSRegion, "AWS_REGION", "us-east-1")
	overlay(&c.JWTSigningKey, "JWT_SIGNING_KEY", "")
	overlay(&c.JWTIssuer, "JWT_ISSUER", "frontend-2-v2")
	overlay(&c.OktaIssuer, "OKTA_ISSUER", "")
	overlay(&c.OktaClientID, "OKTA_CLIENT_ID", "")
	overlay(&c.OktaClientSecret, "OKTA_CLIENT_SECRET", "")
	overlay(&c.OktaRedirectURL, "OKTA_REDIRECT_URL", "")

	if v := os.Getenv("DEV_AUTH_BYPASS"); v == "true" {
		c.DevAuthBypass = true
	}

	if c.JWTSigningKey == "" {
		return nil, errors.New("JWT_SIGNING_KEY is required (set via env or secrets.json)")
	}
	return c, nil
}

func mergeFromFile(c *Config, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, c)
}

func overlay(field *string, envKey, def string) {
	if v := os.Getenv(envKey); v != "" {
		*field = v
		return
	}
	if *field == "" {
		*field = def
	}
}
