package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type Config struct {
	Server      ServerConfig
	DB          DatabaseConfig
	AdminAPIKey string
}

type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

func Load() (*Config, error) {
	environment := getEnv("APP_ENV", "local")
	production := environment == "production"
	defaultPassword := "postgres"
	defaultAdminAPIKey := "local-admin-key"
	defaultSSLMode := "disable"
	if production {
		defaultPassword = ""
		defaultAdminAPIKey = ""
		defaultSSLMode = "require"
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:        getEnv("SERVER_PORT", "8080"),
			ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second,
			IdleTimeout: 60 * time.Second, ShutdownTimeout: 10 * time.Second,
		},
		DB: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", defaultPassword),
			Name:     getEnv("DB_NAME", "sms_gateway"),
			SSLMode:  getEnv("DB_SSLMODE", defaultSSLMode),
		},
		AdminAPIKey: getEnv("ADMIN_API_KEY", defaultAdminAPIKey),
	}
	if cfg.DB.Password == "" {
		return nil, errors.New("DB_PASSWORD is required")
	}
	if cfg.AdminAPIKey == "" {
		return nil, errors.New("ADMIN_API_KEY is required")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
