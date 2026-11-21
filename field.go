package logger

import "log/slog"

// Field is a structured key/value pair attached to a log entry.
//
// Pass fields anywhere in the arguments of the print-style methods. They are
// taken out of the message and written as key/value data, which the console
// format shows as key=value and the JSON format writes as JSON properties:
//
//	log.Info("user signed up", logger.F("user_id", id), logger.F("plan", plan))
//
// Field is an alias for slog.Attr, so slog.String, slog.Int, slog.Duration,
// slog.Group and the other slog constructors work as fields too. The typed
// constructors avoid an allocation for numbers and durations.
type Field = slog.Attr

// ErrorKey is the key used by Err.
const ErrorKey = "error"

// F returns a field with the given key and value.
func F(key string, value any) Field { return slog.Any(key, value) }

// Err returns a field with key ErrorKey holding err. A nil error returns an
// empty field, which is not written.
func Err(err error) Field {
	if err == nil {
		return Field{}
	}
	return slog.Any(ErrorKey, err)
}

// Group returns a field that nests fields under key. The console format writes
// them as key.name=value; the JSON format writes a nested object.
func Group(key string, fields ...Field) Field {
	return Field{Key: key, Value: slog.GroupValue(fields...)}
}
