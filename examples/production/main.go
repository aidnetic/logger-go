// Command production shows a production setup: JSON output, configuration
// from environment variables, redaction, write-error handling, and routing
// the standard log package and log/slog through the logger.
//
//	go run ./production
//	LOG_LEVEL=debug go run ./production
//	LOG_FORMAT=console go run ./production
//	LOG_LEVEL=loud go run ./production     # fails fast with a clear error
package main

import (
	"fmt"
	stdlog "log"
	"log/slog"
	"os"

	"github.com/aidnetic/logger-go"
)

func main() {
	opts := logger.Production() // INFO, JSON, UTC timestamps

	// Never write these, whatever the level or format.
	opts.ReplaceAttr = logger.RedactKeys("password", "token", "authorization", "card_number")

	// Called when an entry cannot be written (disk full, closed pipe). Count
	// it in your metrics; don't log it with the same logger.
	opts.OnError = func(err error) {
		fmt.Fprintln(os.Stderr, "logger: write failed:", err)
	}

	// LOG_LEVEL, LOG_FORMAT, LOG_COLOR and LOG_SOURCE override the preset.
	opts, err := logger.FromEnv(opts)
	if err != nil {
		logger.Fatal("invalid logging configuration:", err)
	}

	// Fields that identify the service go on every entry.
	log := logger.New(opts).With(logger.F("service", "billing"), logger.F("version", "1.4.2"))

	// Make it the default for this package, log/slog and the standard log
	// package, so libraries using either end up in the same JSON stream.
	logger.SetDefault(log)

	log.Info("starting", logger.F("pid", os.Getpid()))
	log.Notice("invoice paid",
		logger.F("invoice_id", "INV-2041"),
		logger.F("amount", 129.00),
		logger.F("card_number", "4242424242424242"), // written as [REDACTED]
	)
	log.Debug("only written with LOG_LEVEL=debug")

	stdlog.Printf("a library using the standard log package")
	slog.Warn("a library using log/slog", "retry", 2)
	logger.Info("the package-level functions use it too")
}
