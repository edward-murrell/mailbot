// Package bootstrap wires the application's dependencies together.
// MakeServer is the single entry point called from main; the other Make
// functions are exported for use in tests or alternative entry points.
package bootstrap

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ekm/mailbot/internal/config"
	"github.com/ekm/mailbot/internal/handler"
	"github.com/ekm/mailbot/internal/mailer"
	"github.com/ekm/mailbot/internal/middleware"
	"github.com/ekm/mailbot/internal/store"
)

// MakeServer assembles all application components and returns a configured
// http.Server. cfg and logger are created in main and passed in.
func MakeServer(cfg *config.Config, logger *slog.Logger) (*http.Server, error) {
	s, err := MakeFileStore(cfg.Storage)
	if err != nil {
		return nil, err
	}
	m := MakeMailer(cfg.SMTP, logger)
	return &http.Server{
		Addr:         cfg.Server.ListenAddr,
		Handler:      makeHandler(cfg, m, s, logger),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}, nil
}

// MakeFileStore ensures the storage directory exists and is writable,
// then assembles a FileStore.
func MakeFileStore(cfg config.StorageConfig) (*store.FileStore, error) {
	info, err := os.Stat(cfg.Dir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
			return nil, fmt.Errorf("create storage dir %s: %w", cfg.Dir, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("stat storage dir %s: %w", cfg.Dir, err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("storage path %s exists but is not a directory", cfg.Dir)
	}

	probe := filepath.Join(cfg.Dir, ".write-test")
	if err := os.WriteFile(probe, nil, 0o644); err != nil {
		return nil, fmt.Errorf("storage dir %s is not writable: %w", cfg.Dir, err)
	}
	_ = os.Remove(probe)

	return store.NewFileStore(cfg), nil
}

// MakeMailer returns an SMTPMailer when SMTP is enabled, or a NoopMailer
// that logs instead of sending (for local development).
func MakeMailer(cfg config.SMTPConfig, logger *slog.Logger) mailer.Mailer {
	if !cfg.Enabled {
		logger.Info("SMTP disabled: using noop mailer")
		return mailer.NewNoopMailer(logger)
	}
	logger.Info("SMTP enabled", "host", cfg.Host, "port", cfg.Port)
	return mailer.NewSMTPMailer(cfg)
}

// makeHandler assembles the middleware chain around the contact handler.
// Chain (outermost → innermost): Recovery → Logger → RateLimiter → mux.
func makeHandler(cfg *config.Config, m mailer.Mailer, s store.Store, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/contact", handler.New(m, s, logger))
	rateLimiter := middleware.NewRateLimiter(time.Duration(cfg.RateLimit.Interval) * time.Second)
	return middleware.Recovery(logger)(
		middleware.Logger(logger)(
			rateLimiter.Middleware(mux),
		),
	)
}
