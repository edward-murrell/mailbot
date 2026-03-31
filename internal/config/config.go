package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Config holds all runtime configuration parsed from environment variables.
// It is immutable after construction via Load.
type Config struct {
	ListenAddr        string `env:"LISTEN_ADDR"         envDefault:":8080"`
	SMTPEnabled       bool   `env:"SMTP_ENABLED"        envDefault:"true"`
	SMTPHost          string `env:"SMTP_HOST"`
	SMTPPort          string `env:"SMTP_PORT"           envDefault:"587"`
	SMTPUser          string `env:"SMTP_USER"`
	SMTPPass          string `env:"SMTP_PASS"`
	SMTPFrom          string `env:"SMTP_FROM"`
	SMTPTo            string `env:"SMTP_TO"`
	SMTPStartTLS      bool   `env:"SMTP_STARTTLS"       envDefault:"true"`
	StorageDir        string `env:"STORAGE_DIR,required"`
	RateLimitInterval int    `env:"RATE_LIMIT_INTERVAL" envDefault:"5"`
	LogLevelRaw string `env:"LOG_LEVEL" envDefault:"info"`
	LogLevel    slog.Level // computed from LogLevelRaw in Load
}

// Load parses environment variables into a Config, applies defaults, and validates.
// All errors are collected and returned together.
func Load() (*Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	var errs []string

	// Parse log level
	switch strings.ToLower(cfg.LogLevelRaw) {
	case "debug":
		cfg.LogLevel = slog.LevelDebug
	case "info":
		cfg.LogLevel = slog.LevelInfo
	case "warn":
		cfg.LogLevel = slog.LevelWarn
	case "error":
		cfg.LogLevel = slog.LevelError
	default:
		errs = append(errs, fmt.Sprintf("LOG_LEVEL: unknown level %q", cfg.LogLevelRaw))
	}

	// Validate rate limit
	if cfg.RateLimitInterval < 1 {
		errs = append(errs, "RATE_LIMIT_INTERVAL: must be a positive integer")
	}

	// SMTP credentials are only required when SMTP is enabled
	if cfg.SMTPEnabled {
		for _, pair := range []struct{ name, val string }{
			{"SMTP_HOST", cfg.SMTPHost},
			{"SMTP_USER", cfg.SMTPUser},
			{"SMTP_PASS", cfg.SMTPPass},
			{"SMTP_FROM", cfg.SMTPFrom},
			{"SMTP_TO", cfg.SMTPTo},
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
