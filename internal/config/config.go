package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Config is the top-level runtime configuration, parsed from environment variables.
// It is immutable after construction via Load.
type Config struct {
	Server    ServerConfig
	SMTP      SMTPConfig      `envPrefix:"SMTP_"`
	Storage   StorageConfig   `envPrefix:"STORAGE_"`
	RateLimit RateLimitConfig `envPrefix:"RATE_LIMIT_"`
	Log       LogConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	ListenAddr string `env:"LISTEN_ADDR" envDefault:":8080"`
}

// SMTPConfig holds outbound mail settings.
type SMTPConfig struct {
	Enabled  bool   `env:"ENABLED"  envDefault:"true"`
	Host     string `env:"HOST"`
	Port     string `env:"PORT"     envDefault:"587"`
	User     string `env:"USER"`
	Pass     string `env:"PASS"`
	From     string `env:"FROM"`
	To       string `env:"TO"`
	StartTLS bool   `env:"STARTTLS" envDefault:"true"`
}

// StorageConfig holds file-storage settings.
type StorageConfig struct {
	Dir string `env:"DIR,required"`
}

// RateLimitConfig holds per-IP rate limiting settings.
type RateLimitConfig struct {
	Interval int `env:"INTERVAL" envDefault:"5"` // seconds between allowed requests per IP
}

// LogConfig holds logging settings.
type LogConfig struct {
	LevelRaw string     `env:"LOG_LEVEL" envDefault:"info"`
	Level    slog.Level // computed from LevelRaw in Load
}

// Load parses environment variables into a Config, applies defaults, and validates.
// All errors are collected and returned together.
func Load() (*Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	var errs []string

	// Parse log level from raw string.
	switch strings.ToLower(cfg.Log.LevelRaw) {
	case "debug":
		cfg.Log.Level = slog.LevelDebug
	case "info":
		cfg.Log.Level = slog.LevelInfo
	case "warn":
		cfg.Log.Level = slog.LevelWarn
	case "error":
		cfg.Log.Level = slog.LevelError
	default:
		errs = append(errs, fmt.Sprintf("LOG_LEVEL: unknown level %q", cfg.Log.LevelRaw))
	}

	// Validate rate limit interval.
	if cfg.RateLimit.Interval < 1 {
		errs = append(errs, "RATE_LIMIT_INTERVAL: must be a positive integer")
	}

	// SMTP credentials are only required when SMTP is enabled.
	if cfg.SMTP.Enabled {
		for _, pair := range []struct{ name, val string }{
			{"SMTP_HOST", cfg.SMTP.Host},
			{"SMTP_USER", cfg.SMTP.User},
			{"SMTP_PASS", cfg.SMTP.Pass},
			{"SMTP_FROM", cfg.SMTP.From},
			{"SMTP_TO", cfg.SMTP.To},
		} {
			if pair.val == "" {
				errs = append(errs, pair.name+" is required when SMTP_ENABLED=true")
			}
		}
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n"))
	}
	return &cfg, nil
}
