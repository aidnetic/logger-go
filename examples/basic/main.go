// Command basic shows the print-style API, the log levels and the
// package-level logger.
//
//	go run ./basic
package main

import (
	"errors"
	"time"

	"github.com/aidnetic/logger-go"
)

func main() {
	// The package-level functions use logger.Default(): INFO and above, console
	// format, colors when standard output is a terminal. Fine for small tools.
	logger.Info("starting up")

	// For services, create a Logger from options and pass it around.
	// Development(): DEBUG level, short timestamps, caller file:line.
	log := logger.New(logger.Development())

	// Print style: arguments are joined like fmt.Println.
	log.Info("listening on", ":8080", "with", 4, "workers")

	// Printf style.
	log.Infof("loaded %d routes in %v", 12, 3*time.Millisecond)

	// The levels, from least to most severe.
	log.Trace("not shown: Development() starts at DEBUG")
	log.Debug("cache warmed in", 120*time.Millisecond)
	log.Notice("first user signed up")
	log.Warn("config file not found, using defaults")
	log.Error("could not reach payment provider:", errors.New("dial tcp 10.0.0.7:443: i/o timeout"))

	// Change the level while the program runs. Safe while other goroutines log.
	log.SetLevel(logger.TRACE)
	log.Trace("shown now")

	// Skip expensive work when the level is off.
	if log.Enabled(logger.DEBUG) {
		log.Debug("state:", expensiveDump())
	}

	// Multi-line messages, such as stack traces, are indented under the message.
	log.Error("job failed: worker crashed\ngoroutine 7 [running]:\nmain.worker()\n\t/app/worker.go:42 +0x1d")

	// Fatal logs, then exits with status 1. Deferred functions do not run.
	// log.Fatal("cannot continue:", err)
}

func expensiveDump() string { return "{connections: 12, queue: 3}" }
