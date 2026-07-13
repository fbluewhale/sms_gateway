package tests

import (
	"testing"
	"time"

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

func TestLoadProviderCircuitBreakerConfiguration(t *testing.T) {
	t.Setenv("SMS_PROVIDER_CIRCUIT_FAILURE_THRESHOLD", "7")
	t.Setenv("SMS_PROVIDER_CIRCUIT_COOLDOWN", "45s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ProviderCircuitFailureThreshold != 7 {
		t.Fatalf("failure threshold = %d, want 7", cfg.ProviderCircuitFailureThreshold)
	}
	if cfg.ProviderCircuitCooldown != 45*time.Second {
		t.Fatalf("cooldown = %s, want 45s", cfg.ProviderCircuitCooldown)
	}
}

func TestLoadRejectsInvalidProviderCircuitBreakerConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		threshold string
		cooldown  string
	}{
		{name: "zero threshold", threshold: "0", cooldown: "30s"},
		{name: "invalid threshold", threshold: "three", cooldown: "30s"},
		{name: "zero cooldown", threshold: "3", cooldown: "0s"},
		{name: "invalid cooldown", threshold: "3", cooldown: "later"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SMS_PROVIDER_CIRCUIT_FAILURE_THRESHOLD", tt.threshold)
			t.Setenv("SMS_PROVIDER_CIRCUIT_COOLDOWN", tt.cooldown)
			if _, err := config.Load(); err == nil {
				t.Fatal("Load() error = nil, want invalid circuit-breaker configuration error")
			}
		})
	}
}
