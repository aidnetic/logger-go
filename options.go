package logger

import (
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"strings"
)

// Format selects how entries are encoded.
type Format int

const (
	// FormatConsole writes human-readable lines, colored when writing to a
	// terminal. It is meant for people: development, CLIs, kubectl logs.
	FormatConsole Format = iota
	// FormatJSON writes one JSON object per line (log/slog's JSON handler).
	// It is meant for log pipelines: Loki, Elasticsearch, CloudWatch, Datadog.
	FormatJSON
	// FormatText writes logfmt key=value lines (log/slog's text handler).
	FormatText
)

func (f Format) String() string {
	switch f {
	case FormatConsole:
		return "console"
	case FormatJSON:
		return "json"
	case FormatText:
		return "text"
	}
	return fmt.Sprintf("Format(%d)", int(f))
}

// ParseFormat converts "console", "json" or "text" (case-insensitive) to a Format.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "console", "pretty":
		return FormatConsole, nil
	case "json":
		return FormatJSON, nil
	case "text", "logfmt":
		return FormatText, nil
	}
	return 0, fmt.Errorf("logger: unknown format %q (want console, json or text)", s)
}

// MarshalText implements encoding.TextMarshaler.
func (f Format) MarshalText() ([]byte, error) { return []byte(f.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler using ParseFormat.
func (f *Format) UnmarshalText(b []byte) error {
	v, err := ParseFormat(string(b))
	if err != nil {
		return err
	}
	*f = v
	return nil
}

// ColorMode controls ANSI colors in the console format.
type ColorMode int

const (
	// ColorAuto uses colors when the output is a terminal. It honors the
	// NO_COLOR and FORCE_COLOR environment variables and TERM=dumb.
	ColorAuto ColorMode = iota
	// ColorAlways always uses colors.
	ColorAlways
	// ColorNever never uses colors.
	ColorNever
)

func (c ColorMode) String() string {
	switch c {
	case ColorAuto:
		return "auto"
	case ColorAlways:
		return "always"
	case ColorNever:
		return "never"
	}
	return fmt.Sprintf("ColorMode(%d)", int(c))
}

// ParseColorMode converts "auto", "always" or "never" to a ColorMode. It also
// accepts true/yes/on/1 for always and false/no/off/0 for never.
func ParseColorMode(s string) (ColorMode, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "auto":
		return ColorAuto, nil
	case "always", "true", "yes", "on", "1":
		return ColorAlways, nil
	case "never", "false", "no", "off", "0":
		return ColorNever, nil
	}
	return 0, fmt.Errorf("logger: unknown color mode %q (want auto, always or never)", s)
}

// MarshalText implements encoding.TextMarshaler.
func (c ColorMode) MarshalText() ([]byte, error) { return []byte(c.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler using ParseColorMode.
func (c *ColorMode) UnmarshalText(b []byte) error {
	v, err := ParseColorMode(string(b))
	if err != nil {
		return err
	}
	*c = v
	return nil
}

// LevelFormat selects how the console format writes level names.
type LevelFormat int

const (
	LevelFull  LevelFormat = iota // INFO, NOTICE, ERROR
	LevelShort                    // INF, NTC, ERR
)

// Layout controls the shape of console lines. The zero value writes fields
// inline after the message, with no padding or truncation.
type Layout struct {
	// LevelFormat selects full or three-letter level names.
	LevelFormat LevelFormat
	// MessageWidth pads short messages to this many characters so that fields
	// line up in a column. 0 turns padding off.
	MessageWidth int
	// MaxValueLen cuts field values longer than this many characters and adds
	// "…". 0 means no limit.
	MaxValueLen int
	// Multiline writes each field on its own indented line, and pretty-prints
	// structs, maps and slices as indented JSON.
	Multiline bool
}

// Options configures a Logger. The zero value is a usable configuration: INFO
// level, console format, standard output, colors when it is a terminal.
type Options struct {
	// Level is the minimum level to write. The zero value is INFO.
	Level Level

	// Format selects the encoding. The zero value is FormatConsole.
	Format Format

	// Output is where entries are written. Nil means os.Stdout.
	//
	// Every entry is written with one Write call, under a lock shared by all
	// Loggers derived from this one, so Output does not have to be safe for
	// concurrent use. Calling New twice with the same writer creates two
	// separate locks; derive Loggers with With instead.
	Output io.Writer

	// Handler replaces the built-in handlers, for example with a vendor's
	// slog.Handler or a slog.NewMultiHandler that fans out to several. When
	// set, Format, Output, TimeFormat, NoTime, UTC, Color, Theme, Layout and
	// ReplaceAttr are ignored; configure the handler itself instead.
	Handler slog.Handler

	// TimeFormat is the timestamp layout (see package time). Empty means
	// DefaultTimeFormat for the console format and RFC 3339 with nanoseconds
	// for JSON and text.
	TimeFormat string

	// NoTime leaves timestamps out, for example when the platform adds its own.
	NoTime bool

	// UTC writes timestamps in UTC instead of local time.
	UTC bool

	// AddSource adds the caller's file and line to every entry. It costs a
	// stack lookup per entry; see the benchmarks in the README.
	AddSource bool

	// Color controls ANSI colors in the console format.
	Color ColorMode

	// Theme sets the console colors. Nil means DefaultTheme().
	Theme *Theme

	// Layout controls the shape of console lines.
	Layout Layout

	// ReplaceAttr rewrites or drops fields before they are written. Use it to
	// redact secrets (see RedactKeys) or rename keys. It is called for each
	// field, with the names of the groups that contain it. For the JSON and text
	// formats it is also called for the built-in time, level, msg and source
	// keys, as in log/slog.
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr

	// OnError is called when an entry cannot be written, for example because
	// the disk is full or a pipe was closed. Nil ignores write errors. It may
	// be called from many goroutines at once.
	OnError func(err error)

	// ExitFunc is called by Fatal and Fatalf after logging. Nil means os.Exit.
	// Tests can replace it to check fatal paths without exiting.
	ExitFunc func(code int)
}

// Development returns options for local development: DEBUG level, colored
// console output with short timestamps, and caller file:line on every entry.
func Development() Options {
	return Options{
		Level:      DEBUG,
		Format:     FormatConsole,
		TimeFormat: timeOnlyMillis,
		AddSource:  true,
	}
}

// Production returns options for production services: INFO level, JSON
// output with UTC timestamps.
func Production() Options {
	return Options{
		Level:  INFO,
		Format: FormatJSON,
		UTC:    true,
	}
}

// New returns a Logger configured by opts.
func New(opts Options) *Logger {
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}
	h := opts.Handler
	if h == nil {
		h = newHandler(out, &opts)
	}
	exit := opts.ExitFunc
	if exit == nil {
		exit = os.Exit
	}
	l := &Logger{
		handler:   h,
		level:     newLevelVar(opts.Level),
		addSource: opts.AddSource,
		exit:      exit,
		onError:   opts.OnError,
	}
	if s, ok := out.(interface{ Sync() error }); ok {
		l.sync = func() { _ = s.Sync() }
	}
	return l
}

// allLevels lets every entry through a handler. Built-in handlers leave
// filtering to the Logger, so that WithLevel can make one Logger more verbose
// than the level the handler was created with.
const allLevels = slog.Level(math.MinInt32)

func newHandler(out io.Writer, o *Options) slog.Handler {
	switch o.Format {
	case FormatJSON, FormatText:
		ho := &slog.HandlerOptions{
			Level:       allLevels,
			AddSource:   o.AddSource,
			ReplaceAttr: builtinReplacer(o),
		}
		if o.Format == FormatJSON {
			return slog.NewJSONHandler(out, ho)
		}
		return slog.NewTextHandler(out, ho)
	default:
		return NewConsoleHandler(out, &ConsoleOptions{
			Level:       allLevels,
			AddSource:   o.AddSource,
			Color:       useColor(o.Color, out),
			TimeFormat:  o.TimeFormat,
			NoTime:      o.NoTime,
			UTC:         o.UTC,
			Theme:       o.Theme,
			Layout:      o.Layout,
			ReplaceAttr: o.ReplaceAttr,
		})
	}
}

// builtinReplacer adapts log/slog's JSON and text handlers: it writes level
// names such as NOTICE and FATAL (slog would write "INFO+2" and "ERROR+4"),
// applies the time options, then calls the user's ReplaceAttr.
func builtinReplacer(o *Options) func([]string, slog.Attr) slog.Attr {
	user, layout, utc, noTime := o.ReplaceAttr, o.TimeFormat, o.UTC, o.NoTime
	return func(groups []string, a slog.Attr) slog.Attr {
		if len(groups) == 0 {
			switch a.Key {
			case slog.LevelKey:
				if lv, ok := a.Value.Any().(slog.Level); ok {
					a.Value = slog.StringValue(Level(lv).String())
				}
			case slog.TimeKey:
				if noTime {
					return slog.Attr{}
				}
				if a.Value.Kind() == slog.KindTime {
					t := a.Value.Time()
					if utc {
						t = t.UTC()
					}
					if layout != "" {
						a.Value = slog.StringValue(t.Format(layout))
					} else {
						a.Value = slog.TimeValue(t)
					}
				}
			}
		}
		if user != nil {
			a = user(groups, a)
		}
		return a
	}
}
