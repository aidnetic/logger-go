package logger_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/aidnetic/logger-go"
)

var fixedTime = time.Date(2026, 11, 14, 10, 44, 34, 1_000_000, time.UTC)

// handle writes one record with a fixed timestamp through h.
func handle(t *testing.T, h slog.Handler, level logger.Level, msg string, attrs ...slog.Attr) {
	t.Helper()
	r := slog.NewRecord(fixedTime, slog.Level(level), msg, 0)
	r.AddAttrs(attrs...)
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}
}

func newConsole(opts logger.ConsoleOptions) (*logger.ConsoleHandler, *bytes.Buffer) {
	var buf bytes.Buffer
	if opts.Level == nil {
		opts.Level = logger.TRACE
	}
	return logger.NewConsoleHandler(&buf, &opts), &buf
}

func TestConsoleLevelsAreAligned(t *testing.T) {
	h, buf := newConsole(logger.ConsoleOptions{UTC: true})
	for _, l := range []logger.Level{logger.TRACE, logger.DEBUG, logger.INFO, logger.NOTICE, logger.WARN, logger.ERROR, logger.FATAL} {
		handle(t, h, l, "message")
	}
	want := "" +
		"2026-11-14 10:44:34.001 TRACE  message\n" +
		"2026-11-14 10:44:34.001 DEBUG  message\n" +
		"2026-11-14 10:44:34.001 INFO   message\n" +
		"2026-11-14 10:44:34.001 NOTICE message\n" +
		"2026-11-14 10:44:34.001 WARN   message\n" +
		"2026-11-14 10:44:34.001 ERROR  message\n" +
		"2026-11-14 10:44:34.001 FATAL  message\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestConsoleTimeOptions(t *testing.T) {
	local := time.FixedZone("UTC+2", 2*60*60)
	r := slog.NewRecord(fixedTime.In(local), slog.LevelInfo, "m", 0)

	var buf bytes.Buffer
	_ = logger.NewConsoleHandler(&buf, &logger.ConsoleOptions{TimeFormat: "15:04:05"}).Handle(context.Background(), r)
	_ = logger.NewConsoleHandler(&buf, &logger.ConsoleOptions{TimeFormat: "15:04:05", UTC: true}).Handle(context.Background(), r)
	_ = logger.NewConsoleHandler(&buf, &logger.ConsoleOptions{NoTime: true}).Handle(context.Background(), r)
	want := "12:44:34 INFO   m\n10:44:34 INFO   m\nINFO   m\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestConsoleValueFormatting(t *testing.T) {
	h, buf := newConsole(logger.ConsoleOptions{NoTime: true})
	type user struct {
		ID   int
		Name string
	}
	handle(t, h, logger.INFO, "values",
		slog.String("plain", "abc"),
		slog.String("spaced", "a b"),
		slog.String("empty", ""),
		slog.String("quote", `say "hi"`),
		slog.Int("int", -3),
		slog.Uint64("uint", 7),
		slog.Float64("float", 1.5),
		slog.Bool("bool", true),
		slog.Duration("took", 1500*time.Millisecond),
		slog.Time("at", fixedTime),
		logger.F("user", user{1, "bob"}),
		logger.F("tags", []string{"a", "b"}),
		logger.F("nil", nil),
		logger.Err(errors.New("connection refused")),
		logger.Err(nil),
	)
	want := `INFO   values  plain=abc spaced="a b" empty="" quote="say \"hi\"" int=-3 uint=7 float=1.5 bool=true ` +
		`took=1.5s at=2026-11-14T10:44:34.001Z user={ID:1 Name:bob} tags=[a b] nil=<nil> error="connection refused"` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestConsoleEscapesUntrustedInput(t *testing.T) {
	h, buf := newConsole(logger.ConsoleOptions{NoTime: true})
	handle(t, h, logger.INFO, "path=/x\x1b[2J\rfake\u202eevil",
		slog.String("header", "a\nINFO forged line"),
		logger.F("raw", struct{ S string }{"\x1b[31mred"}),
	)
	got := buf.String()
	if strings.Contains(got, "\x1b") || strings.Contains(got, "\r") || strings.Contains(got, "\u202e") {
		t.Fatalf("raw control characters reached the output: %q", got)
	}
	if strings.Count(got, "\n") != 1 {
		t.Fatalf("entry spans more than one line: %q", got)
	}
	want := `INFO   path=/x\x1b[2J\rfake\u202eevil  header="a\nINFO forged line" raw={S:\x1b[31mred}` + "\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestConsoleMultiLineMessage(t *testing.T) {
	h, buf := newConsole(logger.ConsoleOptions{TimeFormat: "15:04:05", UTC: true})
	handle(t, h, logger.ERROR, "panic recovered: boom\ngoroutine 1 [running]:\nmain.main()", slog.String("request_id", "r1"))
	want := "" +
		"10:44:34 ERROR  panic recovered: boom  request_id=r1\n" +
		"                goroutine 1 [running]:\n" +
		"                main.main()\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestConsoleGroups(t *testing.T) {
	h, buf := newConsole(logger.ConsoleOptions{NoTime: true})
	var gh slog.Handler = h.WithAttrs([]slog.Attr{slog.String("service", "api")})
	gh = gh.WithGroup("req").WithAttrs([]slog.Attr{slog.String("id", "r1")})
	handle(t, gh, logger.INFO, "grouped",
		logger.Group("http", logger.F("method", "GET"), logger.F("status", 200)),
		logger.Group("empty"),
		logger.Group("", logger.F("inlined", true)),
	)
	want := "INFO   grouped  service=api req.id=r1 req.http.method=GET req.http.status=200 req.inlined=true\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

type secret string

func (secret) LogValue() slog.Value { return slog.StringValue(logger.Redacted) }

type badStringer struct{}

func (*badStringer) String() string { panic("boom") }

func TestConsoleLogValuerAndPanics(t *testing.T) {
	h, buf := newConsole(logger.ConsoleOptions{NoTime: true})
	var nilStringer *badStringer
	handle(t, h, logger.INFO, "safe",
		logger.F("password", secret("hunter2")),
		logger.F("bad", nilStringer),
		logger.F("after", "still written"),
	)
	want := "INFO   safe  password=[REDACTED] bad=<!PANIC: boom> after=\"still written\"\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestConsoleLayout(t *testing.T) {
	t.Run("short levels", func(t *testing.T) {
		h, buf := newConsole(logger.ConsoleOptions{NoTime: true, Layout: logger.Layout{LevelFormat: logger.LevelShort}})
		handle(t, h, logger.NOTICE, "a")
		handle(t, h, logger.INFO+1, "b")
		if got, want := buf.String(), "NTC a\nINF+1 b\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("message width", func(t *testing.T) {
		h, buf := newConsole(logger.ConsoleOptions{NoTime: true, Layout: logger.Layout{MessageWidth: 12}})
		handle(t, h, logger.INFO, "short", slog.Int("a", 1))
		handle(t, h, logger.INFO, "a longer message", slog.Int("b", 2))
		want := "" +
			"INFO   short         a=1\n" +
			"INFO   a longer message  b=2\n"
		if got := buf.String(); got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})
	t.Run("max value length", func(t *testing.T) {
		h, buf := newConsole(logger.ConsoleOptions{NoTime: true, Layout: logger.Layout{MaxValueLen: 5}})
		handle(t, h, logger.INFO, "m", slog.String("body", "abcdefghij"), slog.String("short", "abc"))
		if got, want := buf.String(), "INFO   m  body=abcde… short=abc\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("multiline", func(t *testing.T) {
		h, buf := newConsole(logger.ConsoleOptions{NoTime: true, Layout: logger.Layout{Multiline: true}})
		hh := h.WithAttrs([]slog.Attr{slog.String("request_id", "r1")})
		handle(t, hh, logger.INFO, "user loaded", logger.F("user", map[string]any{"id": 1, "roles": []string{"admin"}}))
		want := "" +
			"INFO   user loaded\n" +
			"       request_id=r1\n" +
			"       user={\n" +
			"         \"id\": 1,\n" +
			"         \"roles\": [\n" +
			"           \"admin\"\n" +
			"         ]\n" +
			"       }\n"
		if got := buf.String(); got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})
}

func TestConsoleColors(t *testing.T) {
	h, buf := newConsole(logger.ConsoleOptions{NoTime: true, Color: true})
	handle(t, h, logger.ERROR, "failed", logger.Err(errors.New("x")), slog.Int("n", 1))
	want := "\x1b[1m\x1b[31mERROR\x1b[0m  failed  " +
		"\x1b[90merror\x1b[0m=\x1b[31mx\x1b[0m " +
		"\x1b[90mn\x1b[0m=1\n"
	if got := buf.String(); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}

	h, buf = newConsole(logger.ConsoleOptions{NoTime: true, Color: false})
	handle(t, h, logger.ERROR, "failed", logger.Err(errors.New("x")))
	if strings.Contains(buf.String(), "\x1b") {
		t.Errorf("colors off but output has escapes: %q", buf.String())
	}
}

func TestConsoleBadgeAndFieldStyle(t *testing.T) {
	theme := logger.BadgeTheme()
	theme.FieldStyle = func(key string, v slog.Value) logger.Style {
		if key == "status" && v.Int64() >= 500 {
			return logger.BrightRed
		}
		return ""
	}
	h, buf := newConsole(logger.ConsoleOptions{NoTime: true, Color: true, Theme: theme})
	handle(t, h, logger.INFO, "GET /", slog.Int("status", 503))
	want := string(logger.Black+logger.BgGreen) + " INFO \x1b[0m   GET /  \x1b[90mstatus\x1b[0m=\x1b[91m503\x1b[0m\n"
	if got := buf.String(); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}

	// v.Int64() panics for a string value; the entry must still be written.
	buf.Reset()
	handle(t, h, logger.INFO, "GET /", slog.String("status", "unknown"))
	if !strings.Contains(buf.String(), "status\x1b[0m=unknown") {
		t.Errorf("panicking FieldStyle broke the entry: %q", buf.String())
	}
}

func TestConsoleReplaceAttrAndRedaction(t *testing.T) {
	h, buf := newConsole(logger.ConsoleOptions{NoTime: true, ReplaceAttr: logger.RedactKeys("password", "Authorization")})
	handle(t, h, logger.INFO, "login",
		slog.String("user", "bob"),
		slog.String("PASSWORD", "hunter2"),
		logger.Group("headers", slog.String("authorization", "Bearer x")),
	)
	want := "INFO   login  user=bob PASSWORD=[REDACTED] headers.authorization=[REDACTED]\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	drop := func(_ []string, a slog.Attr) slog.Attr {
		if a.Key == "debug_only" {
			return slog.Attr{}
		}
		return a
	}
	h, buf = newConsole(logger.ConsoleOptions{NoTime: true, ReplaceAttr: drop})
	handle(t, h, logger.INFO, "m", slog.Int("debug_only", 1), slog.Int("kept", 2))
	if got, want := buf.String(), "INFO   m  kept=2\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestConsoleHandlerLevel(t *testing.T) {
	h := logger.NewConsoleHandler(&bytes.Buffer{}, nil)
	if h.Enabled(context.Background(), slog.LevelDebug) || !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("a nil Level should mean INFO")
	}
}
