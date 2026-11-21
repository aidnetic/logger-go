package logger_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aidnetic/logger-go"
)

func TestJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	opts := logger.Production()
	opts.Output = &buf
	opts.Level = logger.TRACE
	opts.NoTime = true
	log := logger.New(opts)

	log.Notice("order created", logger.F("order_id", 42), logger.F("amount", 19.99))
	log.Trace("t")
	log.With(logger.F("request_id", "r1")).Error("failed", logger.Err(errors.New("timeout")))
	log.Log(logger.INFO+1, "custom")

	want := "" +
		`{"level":"NOTICE","msg":"order created","order_id":42,"amount":19.99}` + "\n" +
		`{"level":"TRACE","msg":"t"}` + "\n" +
		`{"level":"ERROR","msg":"failed","request_id":"r1","error":"timeout"}` + "\n" +
		`{"level":"INFO+1","msg":"custom"}` + "\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestJSONTimeOptions(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Options{Format: logger.FormatJSON, Output: &buf, UTC: true, TimeFormat: time.RFC3339})
	log.Info("m")
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	ts, _ := entry["time"].(string)
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil || !strings.HasSuffix(ts, "Z") {
		t.Fatalf("time %q is not RFC 3339 UTC: %v", ts, err)
	}
	if d := time.Since(parsed); d < 0 || d > time.Minute {
		t.Errorf("timestamp %v is not recent", parsed)
	}
}

func TestJSONRedactionAndSource(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Options{
		Format: logger.FormatJSON, Output: &buf, NoTime: true, AddSource: true,
		ReplaceAttr: logger.RedactKeys("token"),
	})
	log.Info("auth", logger.F("token", "abc"), logger.F("user", "bob"))
	var entry struct {
		Token  string `json:"token"`
		User   string `json:"user"`
		Source struct {
			File string `json:"file"`
			Line int    `json:"line"`
		} `json:"source"`
	}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Token != logger.Redacted || entry.User != "bob" {
		t.Errorf("token=%q user=%q", entry.Token, entry.User)
	}
	if !strings.HasSuffix(entry.Source.File, "options_test.go") || entry.Source.Line == 0 {
		t.Errorf("source = %+v, want this file", entry.Source)
	}
}

func TestTextFormat(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Options{Format: logger.FormatText, Output: &buf, NoTime: true})
	log.Notice("hello world", logger.F("k", "v"))
	if got, want := buf.String(), "level=NOTICE msg=\"hello world\" k=v\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPresets(t *testing.T) {
	d := logger.Development()
	if d.Level != logger.DEBUG || d.Format != logger.FormatConsole || !d.AddSource {
		t.Errorf("Development() = %+v", d)
	}
	p := logger.Production()
	if p.Level != logger.INFO || p.Format != logger.FormatJSON || !p.UTC {
		t.Errorf("Production() = %+v", p)
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv(logger.EnvLevel, "warning")
	t.Setenv(logger.EnvFormat, "text")
	t.Setenv(logger.EnvColor, "never")
	t.Setenv(logger.EnvSource, "true")
	opts, err := logger.FromEnv(logger.Production())
	if err != nil {
		t.Fatal(err)
	}
	if opts.Level != logger.WARN || opts.Format != logger.FormatText || opts.Color != logger.ColorNever || !opts.AddSource || !opts.UTC {
		t.Errorf("FromEnv = %+v", opts)
	}
}

func TestFromEnvUnsetKeepsBase(t *testing.T) {
	for _, k := range []string{logger.EnvLevel, logger.EnvFormat, logger.EnvColor, logger.EnvSource} {
		t.Setenv(k, "")
	}
	base := logger.Development()
	opts, err := logger.FromEnv(base)
	if err != nil || opts.Level != base.Level || opts.Format != base.Format || opts.AddSource != base.AddSource {
		t.Errorf("FromEnv changed the base: %+v, %v", opts, err)
	}
}

func TestFromEnvReportsEveryError(t *testing.T) {
	t.Setenv(logger.EnvLevel, "loud")
	t.Setenv(logger.EnvFormat, "xml")
	t.Setenv(logger.EnvColor, "rainbow")
	t.Setenv(logger.EnvSource, "maybe")
	_, err := logger.FromEnv(logger.Options{})
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, k := range []string{logger.EnvLevel, logger.EnvFormat, logger.EnvColor, logger.EnvSource} {
		if !strings.Contains(err.Error(), k) {
			t.Errorf("error does not mention %s: %v", k, err)
		}
	}
}

func TestFormatAndColorModeText(t *testing.T) {
	var cfg struct {
		Format logger.Format    `json:"format"`
		Color  logger.ColorMode `json:"color"`
	}
	if err := json.Unmarshal([]byte(`{"format":"JSON","color":"always"}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Format != logger.FormatJSON || cfg.Color != logger.ColorAlways {
		t.Errorf("got %+v", cfg)
	}
	if err := json.Unmarshal([]byte(`{"format":"yaml"}`), &cfg); err == nil {
		t.Error("unknown format accepted")
	}
}

func TestColorAuto(t *testing.T) {
	newLog := func() string {
		var buf bytes.Buffer
		logger.New(logger.Options{Output: &buf, NoTime: true}).Info("m")
		return buf.String()
	}
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	if strings.Contains(newLog(), "\x1b") {
		t.Error("a buffer is not a terminal, but output is colored")
	}
	t.Setenv("FORCE_COLOR", "1")
	if !strings.Contains(newLog(), "\x1b") {
		t.Error("FORCE_COLOR did not enable colors")
	}
	t.Setenv("NO_COLOR", "1")
	if strings.Contains(newLog(), "\x1b") {
		t.Error("NO_COLOR did not win over FORCE_COLOR")
	}
}
