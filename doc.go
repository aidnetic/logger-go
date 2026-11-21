// Package logger is a leveled, structured logger with print-style methods and
// human-readable console output, built on log/slog.
//
// # Logging
//
// The print-style methods join their arguments like fmt.Println; the f
// variants format like fmt.Printf:
//
//	log := logger.New(logger.Development())
//	log.Info("server listening on", addr)
//	log.Infof("loaded %d routes in %v", n, elapsed)
//
// Field arguments are taken out of the message and written as structured
// key/value data:
//
//	log.Error("payment failed", logger.Err(err), logger.F("order_id", id))
//
// In the console format that is:
//
//	10:44:34.001 ERROR  payment failed  error="card declined" order_id=8812
//
// and in the JSON format:
//
//	{"time":"2026-11-14T10:44:34.001Z","level":"ERROR","msg":"payment failed","error":"card declined","order_id":8812}
//
// # Levels
//
// TRACE, DEBUG, INFO, NOTICE, WARN, ERROR and FATAL, plus OFF to disable
// logging. The level can be changed at run time with SetLevel, and WithLevel
// gives one request, route or component its own level.
//
// # Request-scoped loggers
//
// With returns a Logger that adds fields to every entry. Store it in a context
// with NewContext and get it back anywhere with FromContext:
//
//	reqLog := log.With(logger.F("request_id", id))
//	ctx = logger.NewContext(ctx, reqLog)
//	...
//	logger.FromContext(ctx).Debug("cache miss for", key)
//
// # Configuration
//
// Options covers level, format (console, JSON, logfmt), output, timestamps,
// caller location, colors, themes, layout and redaction. Development and
// Production return sensible presets, and FromEnv applies LOG_LEVEL,
// LOG_FORMAT, LOG_COLOR and LOG_SOURCE.
//
// # Integrations
//
// Output goes through a slog.Handler. Set Options.Handler to send entries to
// any slog-compatible backend (OpenTelemetry, a vendor SDK, or several at
// once with slog.NewMultiHandler). In the other direction, Logger.Slog and
// Logger.StdLogger hand the Logger to libraries that expect *slog.Logger or
// *log.Logger, and SetDefault routes log/slog and the standard log package
// through it.
//
// The package depends only on the standard library.
package logger
