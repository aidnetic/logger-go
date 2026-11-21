// Command fields shows structured fields: F, Err, Group, With, struct
// values, redaction with slog.LogValuer, the multi-line layout, and LogAttrs
// for hot paths.
//
//	go run ./fields
package main

import (
	"errors"
	"log/slog"
	"time"

	"github.com/aidnetic/logger-go"
)

type User struct {
	ID    int
	Name  string
	Email string
	Roles []string
}

// APIKey hides its value in every log format by implementing slog.LogValuer.
type APIKey string

func (APIKey) LogValue() slog.Value { return slog.StringValue(logger.Redacted) }

func main() {
	log := logger.New(logger.Development())

	// Field arguments become key=value data; everything else is the message.
	log.Info("user signed up", logger.F("user_id", 42), logger.F("plan", "pro"))

	// Fields can sit anywhere among the message arguments.
	log.Info("imported", 1200, "rows", logger.F("table", "orders"))

	// Errors get the "error" key and are highlighted. Err(nil) is skipped.
	log.Error("payment failed", logger.Err(errors.New("card declined")), logger.F("order_id", "A-1009"))

	// The typed slog constructors work as fields too.
	log.Info("request handled", slog.Int("status", 200), slog.Duration("latency", 1830*time.Microsecond))

	// Groups nest fields: http.method=GET http.status=200.
	log.Info("request", logger.Group("http", logger.F("method", "GET"), logger.F("status", 200)))

	// Structs, maps and slices are written readably.
	u := User{ID: 42, Name: "Ada", Email: "ada@example.com", Roles: []string{"admin", "billing"}}
	log.Debug("loaded user", logger.F("user", u))

	// With returns a Logger that adds fields to every entry.
	jobLog := log.With(logger.F("job", "nightly-report"), logger.F("run_id", "r-77"))
	jobLog.Info("started")
	jobLog.Info("finished", logger.F("rows", 18234), logger.F("took", 4200*time.Millisecond))

	// Types can redact themselves.
	log.Info("issued key", logger.F("key", APIKey("sk_live_51H8")))

	// The multi-line layout puts each field on its own line and pretty-prints
	// structs, maps and slices.
	opts := logger.Development()
	opts.Layout = logger.Layout{Multiline: true}
	logger.New(opts).Debug("loaded user", logger.F("user", u), logger.F("cache_hit", true))

	// Hot paths: LogAttrs takes fields without boxing them in ...any, so it
	// does not allocate.
	log.LogAttrs(logger.INFO, "request handled", slog.Int("status", 200), slog.String("path", "/api/users/42"))
}
