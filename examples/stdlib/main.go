// Command stdlib shows how the logger works with the standard library and
// with libraries that expect a *slog.Logger or a *log.Logger.
//
//	go run ./stdlib
package main

import (
	stdlog "log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aidnetic/logger-go"
)

func main() {
	log := logger.New(logger.Development())

	// 1. Route log/slog and the standard log package through the logger.
	logger.SetDefault(log)
	stdlog.Printf("from the standard log package: attempt %d", 1)
	slog.Info("from log/slog", "key", "value")

	// 2. Give a *slog.Logger to a library. It keeps the fields and the level.
	cache := &Cache{log: log.With(logger.F("component", "cache")).Slog()}
	cache.Evict(120)

	// 3. Give a *log.Logger to the standard library, such as
	//    http.Server.ErrorLog, so net/http's own errors are logged at ERROR.
	srv := &http.Server{
		Addr:              "127.0.0.1:0",
		ReadHeaderTimeout: 5 * time.Second,
		ErrorLog:          log.StdLogger(logger.ERROR),
	}
	srv.ErrorLog.Printf("http: TLS handshake error from %s: EOF", "10.0.0.9:51234")

	// 4. Use the console handler with plain log/slog, without this package's
	//    Logger type.
	plain := slog.New(logger.NewConsoleHandler(os.Stdout, &logger.ConsoleOptions{
		Level:      slog.LevelDebug,
		Color:      true,
		TimeFormat: "15:04:05",
	}))
	plain.Debug("plain slog with readable output", "user_id", 42, "plan", "pro")
}

// Cache stands in for a third-party library that accepts a *slog.Logger.
type Cache struct{ log *slog.Logger }

func (c *Cache) Evict(n int) {
	c.log.Info("evicted entries", "count", n)
	c.log.Debug("eviction details", "policy", "lru")
}
