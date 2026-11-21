package logger_test

import (
	"encoding/json"
	"flag"
	"log/slog"
	"testing"

	"github.com/aidnetic/logger-go"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level logger.Level
		want  string
	}{
		{logger.TRACE, "TRACE"},
		{logger.DEBUG, "DEBUG"},
		{logger.INFO, "INFO"},
		{logger.NOTICE, "NOTICE"},
		{logger.WARN, "WARN"},
		{logger.ERROR, "ERROR"},
		{logger.FATAL, "FATAL"},
		{logger.OFF, "OFF"},
		{logger.INFO + 1, "INFO+1"},
		{logger.ERROR + 3, "ERROR+3"},
		{logger.FATAL + 8, "FATAL+8"},
		{logger.TRACE - 2, "TRACE-2"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", int(tt.level), got, tt.want)
		}
	}
}

func TestLevelMatchesSlog(t *testing.T) {
	pairs := map[logger.Level]slog.Level{
		logger.DEBUG: slog.LevelDebug,
		logger.INFO:  slog.LevelInfo,
		logger.WARN:  slog.LevelWarn,
		logger.ERROR: slog.LevelError,
	}
	for l, s := range pairs {
		if l.Level() != s {
			t.Errorf("%v.Level() = %v, want %v", l, l.Level(), s)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in   string
		want logger.Level
	}{
		{"trace", logger.TRACE},
		{"DEBUG", logger.DEBUG},
		{" Info ", logger.INFO},
		{"notice", logger.NOTICE},
		{"warn", logger.WARN},
		{"WARNING", logger.WARN},
		{"error", logger.ERROR},
		{"err", logger.ERROR},
		{"fatal", logger.FATAL},
		{"off", logger.OFF},
		{"none", logger.OFF},
		{"info+1", logger.INFO + 1},
		{"ERROR-2", logger.ERROR - 2},
	}
	for _, tt := range tests {
		got, err := logger.ParseLevel(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, %v; want %v, nil", tt.in, got, err, tt.want)
		}
	}
	for _, bad := range []string{"", "verbose", "info+x", "off+1", "+1"} {
		if _, err := logger.ParseLevel(bad); err == nil {
			t.Errorf("ParseLevel(%q) succeeded, want error", bad)
		}
	}
}

func TestLevelRoundTrip(t *testing.T) {
	for l := logger.TRACE - 4; l <= logger.FATAL+4; l++ {
		got, err := logger.ParseLevel(l.String())
		if err != nil || got != l {
			t.Errorf("ParseLevel(%q) = %v, %v; want %v", l.String(), got, err, l)
		}
	}
}

func TestLevelJSONAndFlag(t *testing.T) {
	var cfg struct {
		Level logger.Level `json:"level"`
	}
	if err := json.Unmarshal([]byte(`{"level":"warning"}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Level != logger.WARN {
		t.Fatalf("unmarshaled level = %v, want WARN", cfg.Level)
	}
	out, err := json.Marshal(cfg)
	if err != nil || string(out) != `{"level":"WARN"}` {
		t.Fatalf("Marshal = %s, %v", out, err)
	}
	if err := json.Unmarshal([]byte(`{"level":"loud"}`), &cfg); err == nil {
		t.Fatal("unmarshaling an unknown level succeeded")
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	lvl := logger.INFO
	fs.Var(&lvl, "log-level", "")
	if err := fs.Parse([]string{"-log-level=debug"}); err != nil || lvl != logger.DEBUG {
		t.Fatalf("flag parse: level=%v err=%v", lvl, err)
	}
}
