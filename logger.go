package logger

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"runtime"
	"sync/atomic"
	"time"
)

// Logger writes log entries. Create one with New and share it: a Logger is
// safe for concurrent use, and every Logger derived from it with With or
// WithLevel writes through the same handler and lock.
//
// The print-style methods (Info, Debug, ...) join their arguments into the
// message the way fmt.Println does, except that Field arguments are taken out
// and written as structured data. The printf-style methods (Infof, Debugf, ...)
// format the message with fmt.Sprintf. go vet checks the arguments of both.
//
// Methods on a nil *Logger use Default(), so a missing logger never panics.
type Logger struct {
	handler   slog.Handler
	level     *levelVar
	addSource bool
	exit      func(int)
	sync      func()
	onError   func(error)
}

// levelVar is a level that can be changed while other goroutines read it.
type levelVar struct{ v atomic.Int64 }

func newLevelVar(l Level) *levelVar {
	lv := &levelVar{}
	lv.v.Store(int64(l))
	return lv
}

func (lv *levelVar) get() Level  { return Level(lv.v.Load()) }
func (lv *levelVar) set(l Level) { lv.v.Store(int64(l)) }

// Trace logs at TRACE level. Arguments are handled as described on Logger.
func (l *Logger) Trace(args ...any) { l.print(TRACE, args...) }

// Debug logs at DEBUG level. Arguments are handled as described on Logger.
func (l *Logger) Debug(args ...any) { l.print(DEBUG, args...) }

// Info logs at INFO level. Arguments are handled as described on Logger.
func (l *Logger) Info(args ...any) { l.print(INFO, args...) }

// Notice logs at NOTICE level. Arguments are handled as described on Logger.
func (l *Logger) Notice(args ...any) { l.print(NOTICE, args...) }

// Warn logs at WARN level. Arguments are handled as described on Logger.
func (l *Logger) Warn(args ...any) { l.print(WARN, args...) }

// Error logs at ERROR level. Arguments are handled as described on Logger.
func (l *Logger) Error(args ...any) { l.print(ERROR, args...) }

// Fatal logs at FATAL level, flushes the output if it has a Sync method, and
// exits the process with status 1 (or calls Options.ExitFunc). Deferred
// functions do not run. FATAL entries are written whatever the level is.
func (l *Logger) Fatal(args ...any) {
	l = l.self()
	l.print(FATAL, args...)
	l.exitProcess()
}

// Log logs at the given level. It never exits, even at FATAL. Use it when the
// level is decided at run time, such as by HTTP status code.
func (l *Logger) Log(level Level, args ...any) { l.print(level, args...) }

// Tracef logs a fmt.Sprintf-formatted message at TRACE level.
func (l *Logger) Tracef(format string, args ...any) { l.printf(TRACE, format, args...) }

// Debugf logs a fmt.Sprintf-formatted message at DEBUG level.
func (l *Logger) Debugf(format string, args ...any) { l.printf(DEBUG, format, args...) }

// Infof logs a fmt.Sprintf-formatted message at INFO level.
func (l *Logger) Infof(format string, args ...any) { l.printf(INFO, format, args...) }

// Noticef logs a fmt.Sprintf-formatted message at NOTICE level.
func (l *Logger) Noticef(format string, args ...any) { l.printf(NOTICE, format, args...) }

// Warnf logs a fmt.Sprintf-formatted message at WARN level.
func (l *Logger) Warnf(format string, args ...any) { l.printf(WARN, format, args...) }

// Errorf logs a fmt.Sprintf-formatted message at ERROR level.
func (l *Logger) Errorf(format string, args ...any) { l.printf(ERROR, format, args...) }

// Fatalf is like Fatal with a fmt.Sprintf-formatted message.
func (l *Logger) Fatalf(format string, args ...any) {
	l = l.self()
	l.printf(FATAL, format, args...)
	l.exitProcess()
}

// Logf logs a fmt.Sprintf-formatted message at the given level. It never exits.
func (l *Logger) Logf(level Level, format string, args ...any) { l.printf(level, format, args...) }

// With returns a Logger that adds fields to every entry. The new Logger shares
// l's level: SetLevel on either one changes both, unless WithLevel is used.
//
//	reqLog := log.With(logger.F("request_id", id))
func (l *Logger) With(fields ...Field) *Logger {
	l = l.self()
	if len(fields) == 0 {
		return l
	}
	c := *l
	c.handler = l.handler.WithAttrs(fields)
	return &c
}

// WithLevel returns a Logger with its own level, independent of l. Use it to
// make one request, route or component more or less verbose than the rest:
//
//	if debugRequested {
//		reqLog = reqLog.WithLevel(logger.DEBUG)
//	}
//
// WithLevel(OFF) returns a Logger that writes nothing except FATAL entries.
func (l *Logger) WithLevel(level Level) *Logger {
	l = l.self()
	c := *l
	c.level = newLevelVar(level)
	return &c
}

// SetLevel changes the minimum level. It is safe to call while other
// goroutines log, and it also affects Loggers derived from l with With.
func (l *Logger) SetLevel(level Level) { l.self().level.set(level) }

// Level returns the current minimum level.
func (l *Logger) Level() Level { return l.self().level.get() }

// Enabled reports whether an entry at level would be written. Use it to skip
// building expensive log arguments:
//
//	if log.Enabled(logger.DEBUG) {
//		log.Debug("state:", dumpState())
//	}
func (l *Logger) Enabled(level Level) bool { return l.self().enabled(level) }

// Slog returns a *slog.Logger that writes through l, with l's fields and level.
// Pass it to libraries that accept a *slog.Logger.
func (l *Logger) Slog() *slog.Logger {
	l = l.self()
	return slog.New(&levelHandler{h: l.handler, level: l.level})
}

// StdLogger returns a *log.Logger that writes each line through l at the given
// level. Use it for APIs that need the standard library type, such as
// http.Server.ErrorLog.
func (l *Logger) StdLogger(level Level) *log.Logger {
	l = l.self()
	return slog.NewLogLogger(&levelHandler{h: l.handler, level: l.level}, slog.Level(level))
}

func (l *Logger) self() *Logger {
	if l == nil || l.handler == nil {
		return Default()
	}
	return l
}

func (l *Logger) enabled(level Level) bool {
	return level >= l.level.get() && l.handler.Enabled(context.Background(), slog.Level(level))
}

// print builds the message the way fmt.Println does, without the newline, and
// takes Field arguments out as structured data.
//
// It must forward args to fmt.Sprintln itself (not through a helper that takes
// a []any) so that go vet recognizes the public methods as print wrappers.
func (l *Logger) print(level Level, args ...any) {
	l = l.self()
	if level < FATAL && !l.enabled(level) {
		return
	}
	if len(args) == 1 {
		if s, ok := args[0].(string); ok {
			l.emit(level, trimNewline(s), nil)
			return
		}
	}
	if hasField(args) {
		var buf [stackFields]Field // keeps the common case off the heap
		msg, fields := splitArgs(args, buf[:0])
		l.emit(level, msg, fields)
		return
	}
	msg := fmt.Sprintln(args...)
	l.emit(level, trimNewline(msg[:len(msg)-1]), nil)
}

// stackFields is how many fields print collects without allocating.
const stackFields = 8

// LogAttrs logs msg with fields at the given level without the allocations
// that passing fields through ...any costs. It is meant for hot paths, such
// as per-request or per-message logging in high-throughput services; the
// print-style methods are fine everywhere else. It never exits.
//
//	log.LogAttrs(logger.INFO, "request handled",
//		slog.Int("status", status), slog.Duration("latency", d))
func (l *Logger) LogAttrs(level Level, msg string, fields ...Field) {
	l.logAttrs(level, msg, fields)
}

func (l *Logger) logAttrs(level Level, msg string, fields []Field) {
	l = l.self()
	if level < FATAL && !l.enabled(level) {
		return
	}
	l.emit(level, msg, fields)
}

func (l *Logger) printf(level Level, format string, args ...any) {
	l = l.self()
	if level < FATAL && !l.enabled(level) {
		return
	}
	l.emit(level, trimNewline(fmt.Sprintf(format, args...)), nil)
}

// callerSkip is the number of frames between runtime.Callers in emit and the
// code that called the logger: runtime.Callers, emit, print or printf, and the
// public method or package-level function.
const callerSkip = 4

func (l *Logger) emit(level Level, msg string, fields []Field) {
	var pc uintptr
	if l.addSource {
		var pcs [1]uintptr
		runtime.Callers(callerSkip, pcs[:])
		pc = pcs[0]
	}
	r := slog.NewRecord(time.Now(), slog.Level(level), msg, pc)
	if len(fields) > 0 {
		r.AddAttrs(fields...)
	}
	if err := l.handler.Handle(context.Background(), r); err != nil && l.onError != nil {
		l.onError(err)
	}
}

func (l *Logger) exitProcess() {
	if l.sync != nil {
		l.sync()
	}
	l.exit(1)
}

func hasField(args []any) bool {
	for _, a := range args {
		if _, ok := a.(Field); ok {
			return true
		}
	}
	return false
}

// splitArgs separates Field arguments, appended to fields, from message
// arguments. It never modifies args: when the caller passes a slice with
// xs..., that slice is theirs.
func splitArgs(args []any, fields []Field) (string, []Field) {
	nonFields := 0
	var last any
	for _, a := range args {
		if f, ok := a.(Field); ok {
			fields = append(fields, f)
		} else {
			nonFields++
			last = a
		}
	}
	switch nonFields {
	case 0:
		return "", fields
	case 1:
		if s, ok := last.(string); ok {
			return trimNewline(s), fields // the common case: one message and some fields
		}
	}
	msgArgs := make([]any, 0, nonFields)
	for _, a := range args {
		if _, ok := a.(Field); !ok {
			msgArgs = append(msgArgs, a)
		}
	}
	msg := fmt.Sprintln(msgArgs...)
	return trimNewline(msg[:len(msg)-1]), fields
}

// trimNewline removes one trailing newline, so messages written for log.Printf
// ("done\n") don't produce an empty continuation line.
func trimNewline(s string) string {
	if n := len(s); n > 0 && s[n-1] == '\n' {
		return s[:n-1]
	}
	return s
}

// levelHandler applies a Logger's level to a slog.Handler, for Slog and StdLogger.
type levelHandler struct {
	h     slog.Handler
	level *levelVar
}

func (h *levelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return Level(level) >= h.level.get() && h.h.Enabled(ctx, level)
}

func (h *levelHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.h.Handle(ctx, r)
}

func (h *levelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelHandler{h: h.h.WithAttrs(attrs), level: h.level}
}

func (h *levelHandler) WithGroup(name string) slog.Handler {
	return &levelHandler{h: h.h.WithGroup(name), level: h.level}
}
