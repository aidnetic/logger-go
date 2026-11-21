// Package logtest records log entries in memory so tests can check what was
// logged.
//
//	func TestCheckout(t *testing.T) {
//		log, rec := logtest.New()
//		svc := NewCheckout(log)
//		svc.Pay(ctx, order)
//
//		e, ok := rec.Find("payment failed")
//		if !ok || e.Fields["order_id"] != order.ID {
//			t.Fatalf("missing payment failure log: %v", rec.Entries())
//		}
//	}
package logtest

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/aidnetic/logger-go"
)

// Entry is one recorded log entry.
type Entry struct {
	Time    time.Time
	Level   logger.Level
	Message string
	// Fields holds the entry's fields, including those added with With.
	// Group members use dotted keys ("http.status"). Values are as slog
	// stores them: int64 for signed integers, uint64 for unsigned,
	// float64, string, bool, time.Duration, time.Time, or the original value.
	Fields map[string]any
}

// String formats the entry for test failure messages.
func (e Entry) String() string {
	return fmt.Sprintf("%s %q %v", e.Level, e.Message, e.Fields)
}

// Recorder is a slog.Handler that keeps every entry in memory. It is safe for
// concurrent use.
type Recorder struct {
	st     *state
	fields []field
	prefix string
}

type field struct {
	key   string
	value any
}

type state struct {
	mu      sync.Mutex
	entries []Entry
	exits   []int
}

// New returns a Logger at TRACE level that records into the returned Recorder.
// Fatal and Fatalf record the exit code instead of exiting, and then return.
func New() (*logger.Logger, *Recorder) {
	r := &Recorder{st: &state{}}
	l := logger.New(logger.Options{
		Level:   logger.TRACE,
		Handler: r,
		ExitFunc: func(code int) {
			r.st.mu.Lock()
			r.st.exits = append(r.st.exits, code)
			r.st.mu.Unlock()
		},
	})
	return l, r
}

// Entries returns a copy of the recorded entries, oldest first.
func (r *Recorder) Entries() []Entry {
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	return append([]Entry(nil), r.st.entries...)
}

// Len returns the number of recorded entries.
func (r *Recorder) Len() int {
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	return len(r.st.entries)
}

// Find returns the first entry whose message contains substr.
func (r *Recorder) Find(substr string) (Entry, bool) {
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	for _, e := range r.st.entries {
		if strings.Contains(e.Message, substr) {
			return e, true
		}
	}
	return Entry{}, false
}

// FindAll returns every entry whose message contains substr.
func (r *Recorder) FindAll(substr string) []Entry {
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	var out []Entry
	for _, e := range r.st.entries {
		if strings.Contains(e.Message, substr) {
			out = append(out, e)
		}
	}
	return out
}

// ExitCodes returns the codes passed to the exit function by Fatal and Fatalf.
func (r *Recorder) ExitCodes() []int {
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	return append([]int(nil), r.st.exits...)
}

// Reset discards all recorded entries and exit codes.
func (r *Recorder) Reset() {
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	r.st.entries = nil
	r.st.exits = nil
}

// Enabled implements slog.Handler. It records every level; the Logger decides
// what reaches it.
func (r *Recorder) Enabled(context.Context, slog.Level) bool { return true }

// Handle implements slog.Handler.
func (r *Recorder) Handle(_ context.Context, rec slog.Record) error {
	fields := make(map[string]any, len(r.fields)+rec.NumAttrs())
	for _, f := range r.fields {
		fields[f.key] = f.value
	}
	rec.Attrs(func(a slog.Attr) bool {
		flatten(r.prefix, a, func(k string, v any) { fields[k] = v })
		return true
	})
	e := Entry{Time: rec.Time, Level: logger.Level(rec.Level), Message: rec.Message, Fields: fields}
	r.st.mu.Lock()
	r.st.entries = append(r.st.entries, e)
	r.st.mu.Unlock()
	return nil
}

// WithAttrs implements slog.Handler.
func (r *Recorder) WithAttrs(attrs []slog.Attr) slog.Handler {
	r2 := *r
	r2.fields = append([]field(nil), r.fields...)
	for _, a := range attrs {
		flatten(r.prefix, a, func(k string, v any) { r2.fields = append(r2.fields, field{k, v}) })
	}
	return &r2
}

// WithGroup implements slog.Handler.
func (r *Recorder) WithGroup(name string) slog.Handler {
	if name == "" {
		return r
	}
	r2 := *r
	r2.prefix = r.prefix + name + "."
	return &r2
}

func flatten(prefix string, a slog.Attr, set func(string, any)) {
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		p := prefix
		if a.Key != "" {
			p += a.Key + "."
		}
		for _, ga := range v.Group() {
			flatten(p, ga, set)
		}
		return
	}
	if a.Key == "" && v.Kind() == slog.KindAny && v.Any() == nil {
		return
	}
	set(prefix+a.Key, v.Any())
}
