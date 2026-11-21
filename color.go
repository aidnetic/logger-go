package logger

import (
	"io"
	"os"
	"strings"
)

// useColor decides whether the console format should use ANSI colors.
func useColor(mode ColorMode, w io.Writer) bool {
	switch mode {
	case ColorAlways:
		return true
	case ColorNever:
		return false
	}
	// https://no-color.org: any non-empty value turns colors off.
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if v := os.Getenv("FORCE_COLOR"); v != "" && v != "0" && !strings.EqualFold(v, "false") {
		return true
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return isTerminal(w)
}

// isTerminal reports whether w is a character device, such as a terminal. It
// uses only the standard library; the check is approximate (/dev/null is also
// a character device), which is harmless for deciding on colors.
func isTerminal(w io.Writer) bool {
	f, ok := w.(interface{ Stat() (os.FileInfo, error) })
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
