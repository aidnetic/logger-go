package logger

import (
	"log/slog"
	"strings"
)

// Redacted replaces the value of fields hidden by RedactKeys.
const Redacted = "[REDACTED]"

// RedactKeys returns a ReplaceAttr function that replaces the value of every
// field whose key matches one of keys (ignoring case) with Redacted:
//
//	opts.ReplaceAttr = logger.RedactKeys("password", "token", "authorization", "card_number")
//
// Only fields are redacted. Text in the message itself cannot be recognized,
// so pass secrets as fields if they might be logged at all. For values of your
// own types, implementing slog.LogValuer is the more robust option.
func RedactKeys(keys ...string) func(groups []string, a slog.Attr) slog.Attr {
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		set[strings.ToLower(k)] = struct{}{}
	}
	return func(_ []string, a slog.Attr) slog.Attr {
		if _, ok := set[strings.ToLower(a.Key)]; ok {
			return slog.String(a.Key, Redacted)
		}
		return a
	}
}
