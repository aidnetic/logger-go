package logger

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
)

// Level is the severity of a log entry. Higher values are more severe.
//
// The values line up with log/slog (DEBUG == slog.LevelDebug, INFO ==
// slog.LevelInfo, WARN == slog.LevelWarn, ERROR == slog.LevelError), so a Level
// converts to and from slog.Level with a plain type conversion. The gaps between
// the named levels leave room for custom levels such as INFO+1.
//
// Level implements encoding.TextMarshaler, encoding.TextUnmarshaler and
// flag.Value (on *Level), so it can be read from JSON/YAML config files,
// environment variables and command-line flags.
type Level int

// Log levels, from least to most severe.
const (
	TRACE  Level = -8 // very detailed diagnostics, usually off even in development
	DEBUG  Level = -4 // diagnostics for developers
	INFO   Level = 0  // normal operation; the zero value and the default
	NOTICE Level = 2  // significant but expected events: user signed up, payment captured
	WARN   Level = 4  // something unexpected that the program recovered from
	ERROR  Level = 8  // a failed operation that needs attention
	FATAL  Level = 12 // the process cannot continue; Fatal and Fatalf exit after logging

	// OFF is a threshold that disables logging. It is not a level to log at.
	OFF Level = math.MaxInt32
)

// levelNames lists the named levels in ascending order.
var levelNames = [...]struct {
	level Level
	name  string
	short string
}{
	{TRACE, "TRACE", "TRC"},
	{DEBUG, "DEBUG", "DBG"},
	{INFO, "INFO", "INF"},
	{NOTICE, "NOTICE", "NTC"},
	{WARN, "WARN", "WRN"},
	{ERROR, "ERROR", "ERR"},
	{FATAL, "FATAL", "FTL"},
}

// levelIndex returns the index of l in levelNames, or -1 if l is not a named level.
func levelIndex(l Level) int {
	switch l {
	case TRACE:
		return 0
	case DEBUG:
		return 1
	case INFO:
		return 2
	case NOTICE:
		return 3
	case WARN:
		return 4
	case ERROR:
		return 5
	case FATAL:
		return 6
	}
	return -1
}

// String returns the level's name, such as "INFO". Levels between the named
// ones are written relative to the named level below them, such as "INFO+1",
// the same way log/slog does.
func (l Level) String() string { return l.format(false) }

func (l Level) format(short bool) string {
	if l == OFF {
		return "OFF"
	}
	for i := len(levelNames) - 1; i >= 0; i-- {
		n := levelNames[i]
		if l < n.level {
			continue
		}
		name := n.name
		if short {
			name = n.short
		}
		if l == n.level {
			return name
		}
		return name + "+" + strconv.Itoa(int(l-n.level))
	}
	name := levelNames[0].name
	if short {
		name = levelNames[0].short
	}
	return name + strconv.Itoa(int(l-TRACE)) // l < TRACE, so this includes the minus sign
}

// ParseLevel converts a level name to a Level. It ignores case and surrounding
// spaces, accepts "warning" for WARN, "err" for ERROR and "none" for OFF, and
// accepts offsets such as "info+1" (the format String produces).
//
// Unknown names return an error instead of a default, so a typo in a config
// file fails at startup rather than silently changing what gets logged.
func ParseLevel(s string) (Level, error) {
	name := strings.TrimSpace(s)
	offset := 0
	if i := strings.IndexAny(name, "+-"); i > 0 {
		n, err := strconv.Atoi(name[i:])
		if err != nil {
			return 0, fmt.Errorf("logger: invalid level %q: bad offset", s)
		}
		name, offset = name[:i], n
	}
	var l Level
	switch strings.ToUpper(name) {
	case "TRACE":
		l = TRACE
	case "DEBUG":
		l = DEBUG
	case "INFO":
		l = INFO
	case "NOTICE":
		l = NOTICE
	case "WARN", "WARNING":
		l = WARN
	case "ERROR", "ERR":
		l = ERROR
	case "FATAL":
		l = FATAL
	case "OFF", "NONE":
		if offset != 0 {
			return 0, fmt.Errorf("logger: invalid level %q: OFF takes no offset", s)
		}
		return OFF, nil
	default:
		return 0, fmt.Errorf("logger: unknown level %q (want trace, debug, info, notice, warn, error, fatal or off)", s)
	}
	return l + Level(offset), nil
}

// Level returns l as a slog.Level. It makes Level a slog.Leveler, so a Level
// can be passed to slog.HandlerOptions and similar.
func (l Level) Level() slog.Level { return slog.Level(l) }

// MarshalText implements encoding.TextMarshaler.
func (l Level) MarshalText() ([]byte, error) { return []byte(l.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler using ParseLevel.
func (l *Level) UnmarshalText(b []byte) error {
	v, err := ParseLevel(string(b))
	if err != nil {
		return err
	}
	*l = v
	return nil
}

// Set implements flag.Value, so a Level can be a command-line flag:
//
//	lvl := logger.INFO
//	flag.Var(&lvl, "log-level", "minimum log level")
func (l *Level) Set(s string) error { return l.UnmarshalText([]byte(s)) }
