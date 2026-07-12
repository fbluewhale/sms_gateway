package tests

import (
	"testing"

	"sms_gateway/internal/config"
)

func TestLoadUsesLocalDevelopmentDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_SSLMODE", "")
	t.Setenv("ADMIN_API_KEY", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DB.Password != "postgres" {
		t.Fatalf("DB password = %q; want local default", cfg.DB.Password)
	}
	if cfg.DB.SSLMode != "disable" {
		t.Fatalf("DB SSL mode = %q; want disable", cfg.DB.SSLMode)
	}
	if cfg.AdminAPIKey != "local-admin-key" {
		t.Fatalf("admin API key = %q; want local default", cfg.AdminAPIKey)
	}
}

func TestLoadRequiresSecretsInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("ADMIN_API_KEY", "")

	if _, err := config.Load(); err == nil {
		t.Fatal("Load() error = nil; want missing secret error")
	}
}

func TestLoadAcceptsProductionSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DB_PASSWORD", "secret-password")
	t.Setenv("ADMIN_API_KEY", "secret-api-key")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DB.SSLMode != "require" {
		t.Fatalf("DB SSL mode = %q; want require", cfg.DB.SSLMode)
	}
}
