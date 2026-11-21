package logger

import (
	"log/slog"
	"strconv"
)

// Style is an ANSI SGR escape sequence that sets text color and attributes.
// Styles are strings, so they combine with +:
//
//	logger.Bold + logger.Yellow
//	logger.BrightWhite + logger.BgRed
//
// The empty Style leaves text unstyled.
type Style string

// Text attributes.
const (
	Reset     Style = "\x1b[0m"
	Bold      Style = "\x1b[1m"
	Dim       Style = "\x1b[2m"
	Italic    Style = "\x1b[3m"
	Underline Style = "\x1b[4m"
)

// Foreground colors.
const (
	Black         Style = "\x1b[30m"
	Red           Style = "\x1b[31m"
	Green         Style = "\x1b[32m"
	Yellow        Style = "\x1b[33m"
	Blue          Style = "\x1b[34m"
	Magenta       Style = "\x1b[35m"
	Cyan          Style = "\x1b[36m"
	White         Style = "\x1b[37m"
	Gray          Style = "\x1b[90m"
	BrightRed     Style = "\x1b[91m"
	BrightGreen   Style = "\x1b[92m"
	BrightYellow  Style = "\x1b[93m"
	BrightBlue    Style = "\x1b[94m"
	BrightMagenta Style = "\x1b[95m"
	BrightCyan    Style = "\x1b[96m"
	BrightWhite   Style = "\x1b[97m"
)

// Background colors.
const (
	BgBlack   Style = "\x1b[40m"
	BgRed     Style = "\x1b[41m"
	BgGreen   Style = "\x1b[42m"
	BgYellow  Style = "\x1b[43m"
	BgBlue    Style = "\x1b[44m"
	BgMagenta Style = "\x1b[45m"
	BgCyan    Style = "\x1b[46m"
	BgWhite   Style = "\x1b[47m"
	BgGray    Style = "\x1b[100m"
)

// Color256 returns a foreground color from the 256-color palette.
func Color256(n uint8) Style { return Style("\x1b[38;5;" + strconv.Itoa(int(n)) + "m") }

// Bg256 returns a background color from the 256-color palette.
func Bg256(n uint8) Style { return Style("\x1b[48;5;" + strconv.Itoa(int(n)) + "m") }

// RGB returns a 24-bit foreground color. Most modern terminals support it.
func RGB(r, g, b uint8) Style {
	return Style("\x1b[38;2;" + strconv.Itoa(int(r)) + ";" + strconv.Itoa(int(g)) + ";" + strconv.Itoa(int(b)) + "m")
}

// BgRGB returns a 24-bit background color.
func BgRGB(r, g, b uint8) Style {
	return Style("\x1b[48;2;" + strconv.Itoa(int(r)) + ";" + strconv.Itoa(int(g)) + ";" + strconv.Itoa(int(b)) + "m")
}

// Theme sets the colors of the console format. Start from DefaultTheme,
// BadgeTheme or MinimalTheme and change what you need:
//
//	theme := logger.DefaultTheme()
//	theme.Info = logger.Bold + logger.RGB(80, 200, 120)
//
// The handler copies the theme when it is created, so changing a Theme
// afterwards has no effect on existing loggers.
type Theme struct {
	// Level label styles. Custom levels use the style of the named level below them.
	Trace, Debug, Info, Notice, Warn, Error, Fatal Style

	// Badge pads level labels with a space on each side, which looks better
	// with background colors: " INFO " instead of "INFO".
	Badge bool

	Time       Style // timestamp
	Message    Style // message text
	Key        Style // field keys
	Value      Style // field values
	ErrorValue Style // field values that are errors
	Source     Style // the source=file:line field

	// FieldStyle, if set, chooses the value style for individual fields, for
	// example to color HTTP status codes by class. key includes group
	// prefixes ("http.status"). Return "" to use the default style.
	FieldStyle func(key string, value slog.Value) Style
}

func (t *Theme) levelStyle(l Level) Style {
	switch {
	case l >= FATAL:
		return t.Fatal
	case l >= ERROR:
		return t.Error
	case l >= WARN:
		return t.Warn
	case l >= NOTICE:
		return t.Notice
	case l >= INFO:
		return t.Info
	case l >= DEBUG:
		return t.Debug
	default:
		return t.Trace
	}
}

// DefaultTheme returns the default colors: bold colored level names, gray
// timestamps and keys, and red error values.
func DefaultTheme() *Theme {
	return &Theme{
		Trace:      Gray,
		Debug:      Bold + Cyan,
		Info:       Bold + Green,
		Notice:     Bold + BrightBlue,
		Warn:       Bold + Yellow,
		Error:      Bold + Red,
		Fatal:      Bold + BrightWhite + BgRed,
		Time:       Gray,
		Key:        Gray,
		ErrorValue: Red,
		Source:     Gray,
	}
}

// BadgeTheme returns a theme with level names on colored backgrounds.
func BadgeTheme() *Theme {
	return &Theme{
		Trace:      BrightWhite + BgGray,
		Debug:      Black + BgCyan,
		Info:       Black + BgGreen,
		Notice:     BrightWhite + BgBlue,
		Warn:       Black + BgYellow,
		Error:      BrightWhite + BgRed,
		Fatal:      Bold + BrightWhite + BgMagenta,
		Badge:      true,
		Time:       Gray,
		Key:        Gray,
		ErrorValue: Red,
		Source:     Gray,
	}
}

// MinimalTheme returns a theme that colors only the level names.
func MinimalTheme() *Theme {
	return &Theme{
		Trace:  Gray,
		Debug:  Cyan,
		Info:   Green,
		Notice: Blue,
		Warn:   Yellow,
		Error:  Red,
		Fatal:  Red,
	}
}
