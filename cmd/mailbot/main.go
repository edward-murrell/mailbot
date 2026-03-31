package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ekm/mailbot/internal/config"
	"github.com/ekm/mailbot/internal/handler"
	"github.com/ekm/mailbot/internal/mailer"
	"github.com/ekm/mailbot/internal/middleware"
	"github.com/ekm/mailbot/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "mailbot: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	fileStore, err := store.NewFileStore(cfg.StorageDir)
	if err != nil {
		return err
	}

	var m mailer.Mailer
	if cfg.SMTPEnabled {
		m = mailer.NewSMTPMailer(mailer.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			User:     cfg.SMTPUser,
			Pass:     cfg.SMTPPass,
			From:     cfg.SMTPFrom,
			To:       cfg.SMTPTo,
			StartTLS: cfg.SMTPStartTLS,
		})
		logger.Info("SMTP enabled", "host", cfg.SMTPHost, "port", cfg.SMTPPort)
	} else {
		m = mailer.NewNoopMailer(logger)
		logger.Info("SMTP disabled: using noop mailer")
	}

	contactHandler := handler.New(m, fileStore, logger)

	mux := http.NewServeMux()
	mux.Handle("/contact", contactHandler)

	// Middleware chain (outermost → innermost):
	// Recovery → Logger → RateLimiter → mux
	rateLimiter := middleware.NewRateLimiter(time.Duration(cfg.RateLimitInterval) * time.Second)
	chain := middleware.Recovery(logger)(
		middleware.Logger(logger)(
			rateLimiter.Middleware(mux),
		),
	)

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      chain,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown on SIGINT / SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		logger.Info("received shutdown signal")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown error", "error", err)
		}
	}()

	logger.Info("starting server", "addr", cfg.ListenAddr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: %w", err)
	}

	<-done
	logger.Info("server stopped")
	return nil
}
