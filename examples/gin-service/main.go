// Command gin-service is a small HTTP API that shows the logger in a gin
// service:
//
//   - a request-scoped logger per request, carrying the request ID, that
//     handlers and deeper code get from the request context
//   - a log level per API route, changeable at run time through admin
//     endpoints: DEBUG for one route, WARN to log only failures, OFF to
//     silence health checks
//   - debug logging for a single request with a header (admin token required)
//   - access logs whose level follows the response status
//   - panic recovery that always logs the stack trace
//   - readable console logs in development, JSON in production
//
// Run it:
//
//	ADMIN_TOKEN=secret go run ./gin-service
//	APP_ENV=production ADMIN_TOKEN=secret go run ./gin-service
//
// examples/README.md has curl commands to try.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/aidnetic/logger-go"
)

func main() {
	log := newLogger(os.Getenv("APP_ENV") == "production")
	logger.SetDefault(log) // log/slog, the std log package and logger.Info use it too

	gin.SetMode(gin.ReleaseMode) // our middleware does the request logging

	levels := NewRouteLevels()
	// Health checks are frequent and uninteresting: never log them.
	levels.Set("GET /healthz", logger.OFF)

	router := newRouter(log, levels, os.Getenv("ADMIN_TOKEN"))

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ErrorLog:          log.StdLogger(logger.ERROR), // net/http's own errors
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("listening on", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server stopped:", err)
		}
	}()

	<-ctx.Done()
	log.Notice("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown did not finish:", err)
	}
	log.Notice("stopped")
}

// newLogger builds the service logger: console output in development, JSON in
// production, overridable with LOG_LEVEL, LOG_FORMAT, LOG_COLOR, LOG_SOURCE.
func newLogger(production bool) *logger.Logger {
	opts := logger.Development()
	if production {
		opts = logger.Production()
	}
	opts.ReplaceAttr = logger.RedactKeys("password", "card_number", "authorization")
	opts.Theme = statusTheme()

	opts, err := logger.FromEnv(opts)
	if err != nil {
		logger.Fatal("invalid logging configuration:", err)
	}
	log := logger.New(opts)
	if production {
		log = log.With(logger.F("service", "orders-api"))
	}
	return log
}

// statusTheme colors HTTP status codes by class in the console format.
func statusTheme() *logger.Theme {
	t := logger.DefaultTheme()
	t.FieldStyle = func(key string, v slog.Value) logger.Style {
		if key != "status" || v.Kind() != slog.KindInt64 {
			return ""
		}
		switch code := v.Int64(); {
		case code >= 500:
			return logger.Bold + logger.Red
		case code >= 400:
			return logger.Yellow
		case code >= 300:
			return logger.Cyan
		default:
			return logger.Green
		}
	}
	return t
}
