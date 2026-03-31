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

	"github.com/ekm/mailbot/internal/bootstrap"
	"github.com/ekm/mailbot/internal/config"
)

func main() {
	cfg, err := config.Load()
	ExitOnErr(err)
	logger := newLogger(cfg.Log.Level)
	srv, srvErr := bootstrap.MakeServer(cfg, logger)
	ExitOnErr(srvErr)
	ExitOnErr(serve(srv, logger))
}

func ExitOnErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "mailbot: %v\n", err)
		os.Exit(1)
	}
}

// serve starts the server and blocks until it shuts down cleanly.
func serve(srv *http.Server, logger *slog.Logger) error {
	ctx := signalContext()

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

	logger.Info("starting server", "addr", srv.Addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: %w", err)
	}

	<-done
	logger.Info("server stopped")
	return nil
}

// signalContext returns a context that is cancelled on SIGINT or SIGTERM.
func signalContext() context.Context {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	defer close(done)
	go func() {
		defer cancel()
		<-ctx.Done()
		println("received shutdown signal")
		time.Sleep(15 * time.Second)
		os.Exit(0)
	}()
	return ctx
}

func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
