// Package platform holds process plumbing shared by every binary: config from
// the environment, structured logging and graceful shutdown.
package platform

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// Env returns the environment variable or a default.
func Env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// EnvInt returns an integer environment variable or a default.
func EnvInt(key string, def int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// EnvDuration returns a duration environment variable (e.g. "30s") or a default.
func EnvDuration(key string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

// NewLogger returns a JSON logger tagged with the service name.
func NewLogger(service string) *slog.Logger {
	level := slog.LevelInfo
	if Env("LOG_LEVEL", "info") == "debug" {
		level = slog.LevelDebug
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(h).With("service", service)
}

// SignalContext is cancelled on SIGINT or SIGTERM.
func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// ServeHTTP runs srv until ctx is cancelled, then shuts it down gracefully.
func ServeHTTP(ctx context.Context, log *slog.Logger, srv *http.Server, grace time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down http server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
