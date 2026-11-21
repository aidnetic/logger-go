package logger

import (
	"log/slog"
	"reflect"
	"sync/atomic"
)

var defaultLogger atomic.Pointer[Logger]

func init() {
	defaultLogger.Store(New(Options{}))
}

// Default returns the package-level Logger used by the top-level functions
// (Info, Errorf, ...), by FromContext when the context has no Logger, and by
// methods called on a nil *Logger. Until SetDefault is called it writes INFO
// and above to standard output in the console format.
func Default() *Logger { return defaultLogger.Load() }

// SetDefault makes l the package-level Logger. It also makes l the default
// for log/slog and the standard log package, so slog.Info and log.Printf
// calls from your code and from libraries go through l (log.Printf at INFO).
//
// Call it once at startup, after creating the Logger from your configuration.
// It panics if l is nil.
func SetDefault(l *Logger) {
	if l == nil {
		panic("logger: SetDefault called with a nil *Logger")
	}
	defaultLogger.Store(l)
	// A Logger that writes through slog's built-in default handler would
	// write back into the log package after this redirect and loop forever.
	if reflect.TypeOf(l.handler).String() != "*slog.defaultHandler" {
		slog.SetDefault(l.Slog())
	}
}

// Trace logs at TRACE level with the default Logger.
func Trace(args ...any) { Default().print(TRACE, args...) }

// Debug logs at DEBUG level with the default Logger.
func Debug(args ...any) { Default().print(DEBUG, args...) }

// Info logs at INFO level with the default Logger.
func Info(args ...any) { Default().print(INFO, args...) }

// Notice logs at NOTICE level with the default Logger.
func Notice(args ...any) { Default().print(NOTICE, args...) }

// Warn logs at WARN level with the default Logger.
func Warn(args ...any) { Default().print(WARN, args...) }

// Error logs at ERROR level with the default Logger.
func Error(args ...any) { Default().print(ERROR, args...) }

// Fatal logs at FATAL level with the default Logger and exits. See Logger.Fatal.
func Fatal(args ...any) {
	l := Default()
	l.print(FATAL, args...)
	l.exitProcess()
}

// Tracef logs a formatted message at TRACE level with the default Logger.
func Tracef(format string, args ...any) { Default().printf(TRACE, format, args...) }

// Debugf logs a formatted message at DEBUG level with the default Logger.
func Debugf(format string, args ...any) { Default().printf(DEBUG, format, args...) }

// Infof logs a formatted message at INFO level with the default Logger.
func Infof(format string, args ...any) { Default().printf(INFO, format, args...) }

// Noticef logs a formatted message at NOTICE level with the default Logger.
func Noticef(format string, args ...any) { Default().printf(NOTICE, format, args...) }

// Warnf logs a formatted message at WARN level with the default Logger.
func Warnf(format string, args ...any) { Default().printf(WARN, format, args...) }

// Errorf logs a formatted message at ERROR level with the default Logger.
func Errorf(format string, args ...any) { Default().printf(ERROR, format, args...) }

// Fatalf logs a formatted message at FATAL level with the default Logger and
// exits. See Logger.Fatal.
func Fatalf(format string, args ...any) {
	l := Default()
	l.printf(FATAL, format, args...)
	l.exitProcess()
}
