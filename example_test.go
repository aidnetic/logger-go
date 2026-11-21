package logger_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/aidnetic/logger-go"
)

// exampleLogger writes to standard output without timestamps or colors, so
// the examples have stable output.
func exampleLogger(opts logger.Options) *logger.Logger {
	opts.Output = os.Stdout
	opts.NoTime = true
	opts.Color = logger.ColorNever
	return logger.New(opts)
}

func Example() {
	log := exampleLogger(logger.Options{})

	log.Info("server listening on", ":8080")
	log.Infof("loaded %d routes in %v", 12, 3*time.Millisecond)
	log.Error("payment failed", logger.Err(errors.New("card declined")), logger.F("order_id", 8812))
	log.Debug("not shown: the default level is INFO")
	// Output:
	// INFO   server listening on :8080
	// INFO   loaded 12 routes in 3ms
	// ERROR  payment failed  error="card declined" order_id=8812
}

func ExampleLogger_With() {
	log := exampleLogger(logger.Options{})

	reqLog := log.With(logger.F("request_id", "7f3a"), logger.F("user_id", 42))
	reqLog.Info("fetching orders")
	reqLog.Warn("slow query", logger.F("took", 1200*time.Millisecond))
	// Output:
	// INFO   fetching orders  request_id=7f3a user_id=42
	// WARN   slow query  request_id=7f3a user_id=42 took=1.2s
}

func ExampleLogger_WithLevel() {
	log := exampleLogger(logger.Options{Level: logger.INFO})

	// One request with debugging turned on, without changing the global level.
	debugLog := log.WithLevel(logger.DEBUG)
	debugLog.Debug("request body:", `{"item":"book"}`)
	log.Debug("not shown")

	// A route that should never log.
	log.WithLevel(logger.OFF).Error("not shown either")
	// Output:
	// DEBUG  request body: {"item":"book"}
}

func ExampleLogger_SetLevel() {
	log := exampleLogger(logger.Options{Level: logger.WARN})
	log.Info("not shown")
	log.SetLevel(logger.INFO) // safe while other goroutines are logging
	log.Info("shown")
	// Output:
	// INFO   shown
}

func ExampleRedactKeys() {
	log := exampleLogger(logger.Options{ReplaceAttr: logger.RedactKeys("password", "card_number")})
	log.Info("login", logger.F("user", "bob"), logger.F("password", "hunter2"))
	// Output:
	// INFO   login  user=bob password=[REDACTED]
}

func ExampleOptions_json() {
	opts := logger.Production()
	opts.Output = os.Stdout
	opts.NoTime = true
	log := logger.New(opts)

	log.Notice("order created", logger.F("order_id", 8812), logger.F("total", 49.95))
	// Output:
	// {"level":"NOTICE","msg":"order created","order_id":8812,"total":49.95}
}

func ExampleGroup() {
	log := exampleLogger(logger.Options{})
	log.Info("request", logger.Group("http", logger.F("method", "GET"), logger.F("status", 200)))
	// Output:
	// INFO   request  http.method=GET http.status=200
}

func ExampleLayout_multiline() {
	log := exampleLogger(logger.Options{Layout: logger.Layout{Multiline: true}})
	log.Info("user loaded", logger.F("user", map[string]any{"id": 1, "name": "bob"}), logger.F("cached", true))
	// Output:
	// INFO   user loaded
	//        user={
	//          "id": 1,
	//          "name": "bob"
	//        }
	//        cached=true
}

func ExampleLogger_Slog() {
	log := exampleLogger(logger.Options{})

	// Hand the logger to a library that takes a *slog.Logger.
	lib := log.With(logger.F("component", "cache")).Slog()
	lib.Info("evicted", "keys", 120)
	lib.Log(context.Background(), slog.Level(logger.NOTICE), "resized", "size_mb", 512)
	// Output:
	// INFO   evicted  component=cache keys=120
	// NOTICE resized  component=cache size_mb=512
}
