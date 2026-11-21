package logger_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aidnetic/logger-go"
	"github.com/aidnetic/logger-go/logtest"
)

// newTestLogger returns a console Logger without timestamps or colors that
// writes to a buffer.
func newTestLogger(opts logger.Options) (*logger.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	opts.Output = &buf
	opts.NoTime = true
	if opts.Color == logger.ColorAuto {
		opts.Color = logger.ColorNever
	}
	return logger.New(opts), &buf
}

// useDefault makes l the default Logger for the duration of the test and
// restores the previous package, slog and std log defaults afterwards.
func useDefault(t *testing.T, l *logger.Logger) {
	t.Helper()
	prev, prevSlog := logger.Default(), slog.Default()
	logger.SetDefault(l)
	t.Cleanup(func() {
		logger.SetDefault(prev)
		slog.SetDefault(prevSlog)
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	})
}

func TestPrintStyle(t *testing.T) {
	log, buf := newTestLogger(logger.Options{})
	log.Info("server listening on", ":8080")
	log.Info("loaded", 12, "routes in", 3*time.Millisecond)
	log.Info()
	log.Info("trailing newline is trimmed\n")
	want := "" +
		"INFO   server listening on :8080\n" +
		"INFO   loaded 12 routes in 3ms\n" +
		"INFO   \n" +
		"INFO   trailing newline is trimmed\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFieldsAreTakenOutOfTheMessage(t *testing.T) {
	log, buf := newTestLogger(logger.Options{})
	log.Info("user signed up", logger.F("user_id", 42), logger.F("plan", "pro"))
	log.Info("loaded", 3, logger.F("source", "db"), "items")
	log.Info(logger.F("only", "fields"))
	log.Warn("disk almost full", slog.Int("percent", 93))
	want := "" +
		"INFO   user signed up  user_id=42 plan=pro\n" +
		"INFO   loaded 3 items  source=db\n" +
		"INFO   only=fields\n" +
		"WARN   disk almost full  percent=93\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCallerSliceIsNotModified(t *testing.T) {
	log, _ := newTestLogger(logger.Options{})
	args := []any{"a", logger.F("k", 1), "b", logger.F("j", 2)}
	snapshot := fmt.Sprint(args...)
	log.Info(args...)
	if got := fmt.Sprint(args...); got != snapshot {
		t.Errorf("args changed: %s, was %s", got, snapshot)
	}
}

func TestPrintfStyle(t *testing.T) {
	log, buf := newTestLogger(logger.Options{Level: logger.TRACE})
	log.Tracef("t %d", 1)
	log.Debugf("d %d", 2)
	log.Infof("i %s", "three")
	log.Noticef("n %v", true)
	log.Warnf("w %.1f", 4.5)
	log.Errorf("e %q", "six")
	log.Logf(logger.INFO+1, "custom %d", 7)
	log.Infof("old habit\n")
	want := "" +
		"TRACE  t 1\n" +
		"DEBUG  d 2\n" +
		"INFO   i three\n" +
		"NOTICE n true\n" +
		"WARN   w 4.5\n" +
		"ERROR  e \"six\"\n" +
		"INFO+1 custom 7\n" +
		"INFO   old habit\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestLevelFiltering(t *testing.T) {
	log, rec := logtest.New()
	log.SetLevel(logger.WARN)
	log.Debug("hidden")
	log.Info("hidden")
	log.Infof("hidden %d", 1)
	log.Warn("shown")
	log.Error("shown")
	if got := rec.Len(); got != 2 {
		t.Fatalf("recorded %d entries, want 2: %v", got, rec.Entries())
	}
	if log.Enabled(logger.INFO) || !log.Enabled(logger.ERROR) {
		t.Error("Enabled does not match the level")
	}
	if log.Level() != logger.WARN {
		t.Errorf("Level() = %v, want WARN", log.Level())
	}
}

func TestWithSharesLevelWithLevelDoesNot(t *testing.T) {
	root, rec := logtest.New()
	root.SetLevel(logger.INFO)
	child := root.With(logger.F("component", "db"))
	verbose := root.WithLevel(logger.DEBUG)
	quiet := child.WithLevel(logger.OFF)

	child.Debug("hidden: shares INFO with root")
	verbose.Debug("shown: own DEBUG level")
	quiet.Error("hidden: OFF")

	root.SetLevel(logger.DEBUG) // affects root and child, not verbose or quiet
	child.Debug("shown: root lowered to DEBUG")
	verbose.SetLevel(logger.ERROR)
	root.Info("shown: root unaffected by verbose.SetLevel")

	var got []string
	for _, e := range rec.Entries() {
		got = append(got, e.Message)
	}
	want := []string{"shown: own DEBUG level", "shown: root lowered to DEBUG", "shown: root unaffected by verbose.SetLevel"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("messages = %q, want %q", got, want)
	}
	if e, _ := rec.Find("root lowered"); e.Fields["component"] != "db" {
		t.Errorf("child lost its field: %v", e)
	}
}

func TestWithAccumulatesFields(t *testing.T) {
	log, buf := newTestLogger(logger.Options{})
	reqLog := log.With(logger.F("request_id", "r-1")).With(logger.F("user_id", 7))
	reqLog.Info("done", logger.F("status", 200))
	log.Info("root has no fields")
	want := "" +
		"INFO   done  request_id=r-1 user_id=7 status=200\n" +
		"INFO   root has no fields\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFatalAlwaysLogsAndExits(t *testing.T) {
	log, rec := logtest.New()
	log.SetLevel(logger.OFF)
	log.Fatal("cannot start:", errors.New("port in use"))
	log.Fatalf("second %d", 2)
	if got := rec.ExitCodes(); len(got) != 2 || got[0] != 1 {
		t.Fatalf("exit codes = %v, want [1 1]", got)
	}
	e, ok := rec.Find("cannot start: port in use")
	if !ok || e.Level != logger.FATAL {
		t.Fatalf("fatal entry missing: %v", rec.Entries())
	}
	// Log at FATAL writes but never exits.
	log.Log(logger.FATAL, "no exit")
	if got := len(rec.ExitCodes()); got != 2 {
		t.Errorf("Log(FATAL) called exit; exit count = %d", got)
	}
}

type syncWriter struct {
	bytes.Buffer
	synced atomic.Bool
}

func (w *syncWriter) Sync() error { w.synced.Store(true); return nil }

func TestFatalSyncsOutput(t *testing.T) {
	var w syncWriter
	code := -1
	log := logger.New(logger.Options{Output: &w, NoTime: true, Color: logger.ColorNever, ExitFunc: func(c int) { code = c }})
	log.Fatal("bye")
	if !w.synced.Load() || code != 1 {
		t.Errorf("synced=%v code=%d, want true 1", w.synced.Load(), code)
	}
	if got := w.String(); got != "FATAL  bye\n" {
		t.Errorf("output = %q", got)
	}
}

func TestSourcePointsAtCaller(t *testing.T) {
	log, buf := newTestLogger(logger.Options{AddSource: true})
	useDefault(t, log)

	check := func(name string, logFn func(), line int) {
		t.Helper()
		buf.Reset()
		logFn()
		// The directory part depends on where the module is checked out.
		want := fmt.Sprintf("/logger_test.go:%d\n", line)
		if !strings.HasSuffix(buf.String(), want) || !strings.Contains(buf.String(), " source=") {
			t.Errorf("%s: got %q, want suffix %q", name, buf.String(), want)
		}
	}
	_, _, line, _ := runtime.Caller(0)
	check("method", func() { log.Info("x") }, line+1)
	check("printf method", func() { log.Infof("x %d", 1) }, line+2)
	check("Log", func() { log.Log(logger.WARN, "x") }, line+3)
	check("with fields", func() { log.Info("x", logger.F("k", 1)) }, line+4)
	check("package func", func() { logger.Info("x") }, line+5)
	check("package printf", func() { logger.Errorf("x %d", 1) }, line+6)
	check("nil logger", func() { (*logger.Logger)(nil).Info("x") }, line+7)
	check("LogAttrs", func() { log.LogAttrs(logger.INFO, "x", slog.Int("k", 1)) }, line+8)
}

func TestLogAttrs(t *testing.T) {
	log, rec := logtest.New()
	log.SetLevel(logger.INFO)
	log.LogAttrs(logger.DEBUG, "hidden")
	log.LogAttrs(logger.WARN, "slow", slog.Int("ms", 1200), logger.F("route", "/x"))
	e, ok := rec.Find("slow")
	if rec.Len() != 1 || !ok || e.Level != logger.WARN || e.Fields["ms"] != int64(1200) || e.Fields["route"] != "/x" {
		t.Fatalf("entries = %v", rec.Entries())
	}
}

// TestAllocations pins the allocation counts that the benchmarks report, so a
// change that adds allocations to a hot path fails the tests.
func TestAllocations(t *testing.T) {
	log := logger.New(logger.Options{Output: discard{}, Color: logger.ColorNever})
	tests := []struct {
		name string
		max  float64
		fn   func()
	}{
		{"message", 0, func() { log.Info("request handled") }},
		{"LogAttrs with 3 fields", 0, func() {
			log.LogAttrs(logger.INFO, "request handled", slog.Int("status", 200), slog.String("path", "/x"), slog.Bool("ok", true))
		}},
		// Passing a Field through ...any boxes it: one allocation per field.
		{"print with 3 fields", 3, func() {
			log.Info("request handled", slog.Int("status", 200), slog.String("path", "/x"), slog.Bool("ok", true))
		}},
		{"disabled message", 0, func() { log.Debug("hidden") }},
		{"disabled printf", 0, func() { log.Debugf("hidden %d", 1) }},
		{"disabled LogAttrs", 0, func() { log.LogAttrs(logger.DEBUG, "hidden", slog.Int("status", 200)) }},
	}
	for _, tt := range tests {
		if got := testing.AllocsPerRun(200, tt.fn); got > tt.max {
			t.Errorf("%s: %v allocations, want at most %v", tt.name, got, tt.max)
		}
	}
}

func TestNilLoggerUsesDefault(t *testing.T) {
	log, buf := newTestLogger(logger.Options{})
	useDefault(t, log)
	var nilLog *logger.Logger
	nilLog.Info("via default")
	nilLog.With(logger.F("k", "v")).Warn("still default")
	want := "INFO   via default\nWARN   still default  k=v\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSetDefaultRoutesStdLogAndSlog(t *testing.T) {
	log, buf := newTestLogger(logger.Options{})
	useDefault(t, log)
	logPrint("from the standard log package")
	slog.Info("from slog", "attempt", 2)
	slog.Debug("hidden by level")
	logger.Notice("from the package-level function")
	want := "" +
		"INFO   from the standard log package\n" +
		"INFO   from slog  attempt=2\n" +
		"NOTICE from the package-level function\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// logPrint calls log.Print; kept separate so the test reads clearly.
func logPrint(v ...any) { log.Print(v...) }

func TestSlogAndStdLoggerBridges(t *testing.T) {
	log, buf := newTestLogger(logger.Options{})
	l := log.With(logger.F("component", "billing"))

	s := l.Slog()
	s.Log(context.Background(), slog.Level(logger.NOTICE), "invoice sent", "invoice", "INV-1")
	s.Debug("hidden")
	log.SetLevel(logger.DEBUG)
	s.Debug("shown after SetLevel on the root")

	std := l.StdLogger(logger.ERROR)
	std.Printf("http: TLS handshake error from %s", "10.0.0.1:5000")

	want := "" +
		"NOTICE invoice sent  component=billing invoice=INV-1\n" +
		"DEBUG  shown after SetLevel on the root  component=billing\n" +
		"ERROR  http: TLS handshake error from 10.0.0.1:5000  component=billing\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestContext(t *testing.T) {
	log, rec := logtest.New()
	reqLog := log.With(logger.F("request_id", "abc"))
	ctx := logger.NewContext(context.Background(), reqLog)
	logger.FromContext(ctx).Info("deep in the call stack")
	if e, ok := rec.Find("deep"); !ok || e.Fields["request_id"] != "abc" {
		t.Fatalf("context logger not used: %v", rec.Entries())
	}
	if logger.FromContext(context.Background()) != logger.Default() {
		t.Error("FromContext without a Logger should return Default()")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestOnError(t *testing.T) {
	var got error
	log := logger.New(logger.Options{Output: failingWriter{}, OnError: func(err error) { got = err }})
	log.Info("lost")
	if got == nil || got.Error() != "disk full" {
		t.Errorf("OnError got %v", got)
	}
}

// overlapWriter counts Write calls that run at the same time.
type overlapWriter struct {
	inFlight, overlaps, writes atomic.Int64
	mu                         sync.Mutex
	lines                      []string
}

func (w *overlapWriter) Write(p []byte) (int, error) {
	if w.inFlight.Add(1) > 1 {
		w.overlaps.Add(1)
	}
	time.Sleep(time.Microsecond)
	w.mu.Lock()
	w.lines = append(w.lines, string(p))
	w.mu.Unlock()
	w.writes.Add(1)
	w.inFlight.Add(-1)
	return len(p), nil
}

func TestConcurrentWritesNeverOverlap(t *testing.T) {
	for _, format := range []logger.Format{logger.FormatConsole, logger.FormatJSON} {
		t.Run(format.String(), func(t *testing.T) {
			w := &overlapWriter{}
			root := logger.New(logger.Options{Output: w, Format: format, Level: logger.DEBUG, Color: logger.ColorNever})
			children := []*logger.Logger{
				root,
				root.With(logger.F("a", 1)),
				root.WithLevel(logger.TRACE),
				root.With(logger.F("b", 2)).With(logger.F("c", 3)),
			}
			var wg sync.WaitGroup
			for g := 0; g < 40; g++ {
				wg.Add(1)
				go func(g int) {
					defer wg.Done()
					l := children[g%len(children)]
					for i := 0; i < 25; i++ {
						l.Info("msg", logger.F("g", g), logger.F("i", i))
						if i%5 == 0 {
							root.SetLevel(logger.DEBUG) // concurrent level writes must be safe
						}
					}
				}(g)
			}
			wg.Wait()
			if n := w.overlaps.Load(); n != 0 {
				t.Errorf("%d overlapping writes", n)
			}
			if n := w.writes.Load(); n != 1000 {
				t.Errorf("%d writes, want 1000", n)
			}
			for _, line := range w.lines {
				if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
					t.Fatalf("entry is not exactly one line: %q", line)
				}
			}
		})
	}
}
