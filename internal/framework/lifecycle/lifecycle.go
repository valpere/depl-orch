// Package lifecycle provides coordinated startup and graceful shutdown
// for services. Services are started in registration order and stopped
// in reverse order.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Service is a long-running component managed by the App.
// Start blocks until the service finishes (or ctx is cancelled).
// Stop is called with a deadline context during shutdown.
type Service interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// HealthReporter is optionally implemented by services that expose health.
type HealthReporter interface {
	Health(ctx context.Context) HealthStatus
}

// HealthStatus is a point-in-time health snapshot.
type HealthStatus struct {
	Name    string    `json:"name"`
	Healthy bool      `json:"healthy"`
	Message string    `json:"message,omitempty"`
	At      time.Time `json:"at"`
}

// Closer is a resource that needs cleanup after services stop.
type Closer interface {
	Close(ctx context.Context) error
}

// Option configures the App.
type Option func(*appConfig)

type appConfig struct {
	shutdownTimeout time.Duration
}

// WithShutdownTimeout sets the maximum time allowed for graceful shutdown.
// Default is 10 seconds.
func WithShutdownTimeout(d time.Duration) Option {
	return func(c *appConfig) { c.shutdownTimeout = d }
}

// App manages a set of services with coordinated lifecycle.
type App struct {
	name     string
	logger   *slog.Logger
	cfg      appConfig
	services []Service
	closers  []Closer
}

// New creates a new App.
func New(name string, logger *slog.Logger, opts ...Option) *App {
	cfg := appConfig{shutdownTimeout: 10 * time.Second}
	for _, o := range opts {
		o(&cfg)
	}
	return &App{
		name:   name,
		logger: logger,
		cfg:    cfg,
	}
}

// Register adds a service. Services start in registration order
// and stop in reverse order.
func (a *App) Register(svc Service) {
	a.services = append(a.services, svc)
}

// RegisterCloser adds a resource that is closed after all services stop.
func (a *App) RegisterCloser(c Closer) {
	a.closers = append(a.closers, c)
}

// Run starts all services and blocks until a termination signal (SIGINT/SIGTERM)
// or the parent context is cancelled. Services are started concurrently;
// if any service returns an error, all others are stopped.
func (a *App) Run(ctx context.Context) error {
	a.logger.Info("app starting", "name", a.name, "services", len(a.services))

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Start all services concurrently.
	errCh := make(chan error, len(a.services))
	var wg sync.WaitGroup

	for _, svc := range a.services {
		wg.Go(func() {
			a.logger.Info("service starting", "service", svc.Name())
			if err := svc.Start(ctx); err != nil && ctx.Err() == nil {
				a.logger.Error("service failed", "service", svc.Name(), "error", err)
				errCh <- fmt.Errorf("service %s: %w", svc.Name(), err)
				cancel() // trigger shutdown of other services
			}
		})
	}

	// Wait for context cancellation (signal or service failure).
	<-ctx.Done()
	a.logger.Info("shutdown initiated", "name", a.name)

	// Stop services in reverse order with timeout.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), a.cfg.shutdownTimeout)
	defer shutdownCancel()

	var shutdownErrs []error
	for i := len(a.services) - 1; i >= 0; i-- {
		svc := a.services[i]
		a.logger.Info("stopping service", "service", svc.Name())
		if err := svc.Stop(shutdownCtx); err != nil {
			a.logger.Error("service stop error", "service", svc.Name(), "error", err)
			shutdownErrs = append(shutdownErrs, fmt.Errorf("stop %s: %w", svc.Name(), err))
		}
	}

	// Wait for all Start goroutines to return.
	wg.Wait()

	// Close additional resources.
	for _, c := range a.closers {
		if err := c.Close(shutdownCtx); err != nil {
			shutdownErrs = append(shutdownErrs, err)
		}
	}

	// Collect any service start errors.
	close(errCh)
	for err := range errCh {
		shutdownErrs = append(shutdownErrs, err)
	}

	if len(shutdownErrs) > 0 {
		return errors.Join(shutdownErrs...)
	}

	a.logger.Info("app stopped", "name", a.name)
	return nil
}

// CloserFunc adapts a plain function into a Closer.
type CloserFunc func(ctx context.Context) error

// Close implements Closer.
func (f CloserFunc) Close(ctx context.Context) error { return f(ctx) }

// Health returns health status for all services that implement HealthReporter.
func (a *App) Health(ctx context.Context) []HealthStatus {
	var statuses []HealthStatus
	for _, svc := range a.services {
		if hr, ok := svc.(HealthReporter); ok {
			statuses = append(statuses, hr.Health(ctx))
		}
	}
	return statuses
}
