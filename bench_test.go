package logger_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/aidnetic/logger-go"
)

// discard is a no-op writer. It is not io.Discard, because some loggers
// detect io.Discard and skip formatting, which would make the numbers
// meaningless.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func benchLogger(opts logger.Options) *logger.Logger {
	opts.Output = discard{}
	opts.Color = logger.ColorNever
	return logger.New(opts)
}

func BenchmarkConsole(b *testing.B) {
	b.Run("message", func(b *testing.B) {
		log := benchLogger(logger.Options{})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Info("request handled")
		}
	})
	b.Run("print args", func(b *testing.B) {
		log := benchLogger(logger.Options{})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Info("user", 42, "logged in from", "10.0.0.1")
		}
	})
	b.Run("printf", func(b *testing.B) {
		log := benchLogger(logger.Options{})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Infof("user %d logged in from %s", 42, "10.0.0.1")
		}
	})
	b.Run("3 fields", func(b *testing.B) {
		log := benchLogger(logger.Options{})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Info("request handled", logger.F("status", 200), logger.F("path", "/api/users/42"), logger.F("latency", 1234*time.Microsecond))
		}
	})
	b.Run("3 typed fields", func(b *testing.B) {
		log := benchLogger(logger.Options{})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Info("request handled", slog.Int("status", 200), slog.String("path", "/api/users/42"), slog.Duration("latency", 1234*time.Microsecond))
		}
	})
	b.Run("LogAttrs 3 fields", func(b *testing.B) {
		log := benchLogger(logger.Options{})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.LogAttrs(logger.INFO, "request handled", slog.Int("status", 200), slog.String("path", "/api/users/42"), slog.Duration("latency", 1234*time.Microsecond))
		}
	})
	b.Run("with logger + 1 field", func(b *testing.B) {
		log := benchLogger(logger.Options{}).With(logger.F("request_id", "7f3a9c"), logger.F("user_id", 42))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Info("request handled", slog.Int("status", 200))
		}
	})
	b.Run("source", func(b *testing.B) {
		log := benchLogger(logger.Options{AddSource: true})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Info("request handled", slog.Int("status", 200))
		}
	})
	b.Run("colors", func(b *testing.B) {
		log := logger.New(logger.Options{Output: discard{}, Color: logger.ColorAlways})
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Info("request handled", slog.Int("status", 200), slog.String("path", "/api/users/42"))
		}
	})
}

func BenchmarkDisabled(b *testing.B) {
	log := benchLogger(logger.Options{Level: logger.INFO})
	b.Run("message", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Debug("request handled")
		}
	})
	b.Run("printf", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Debugf("user %d logged in", 42)
		}
	})
	b.Run("3 typed fields", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.Debug("request handled", slog.Int("status", 200), slog.String("path", "/api/users/42"), slog.Duration("latency", time.Millisecond))
		}
	})
}

func BenchmarkJSON(b *testing.B) {
	log := benchLogger(logger.Production())
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		log.Info("request handled", slog.Int("status", 200), slog.String("path", "/api/users/42"), slog.Duration("latency", 1234*time.Microsecond))
	}
}

func BenchmarkParallel(b *testing.B) {
	log := benchLogger(logger.Options{}).With(logger.F("service", "api"))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			log.Info("request handled", slog.Int("status", 200), slog.String("path", "/api/users/42"))
		}
	})
}

// Baselines: log/slog's own handlers with the same entry.
func BenchmarkSlogBaseline(b *testing.B) {
	b.Run("text", func(b *testing.B) {
		log := slog.New(slog.NewTextHandler(discard{}, nil))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.LogAttrs(context.Background(), slog.LevelInfo, "request handled", slog.Int("status", 200), slog.String("path", "/api/users/42"), slog.Duration("latency", 1234*time.Microsecond))
		}
	})
	b.Run("json", func(b *testing.B) {
		log := slog.New(slog.NewJSONHandler(discard{}, nil))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			log.LogAttrs(context.Background(), slog.LevelInfo, "request handled", slog.Int("status", 200), slog.String("path", "/api/users/42"), slog.Duration("latency", 1234*time.Microsecond))
		}
	})
}
