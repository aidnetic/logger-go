package logger

import "context"

type ctxKey struct{}

// NewContext returns a copy of ctx that carries l. Store a request-scoped
// Logger (with a request ID, user ID, or a per-route level) in the request's
// context, and code deeper in the call stack gets it back with FromContext.
func NewContext(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the Logger stored in ctx by NewContext, or Default() if
// there is none. It never returns nil.
func FromContext(ctx context.Context) *Logger {
	if ctx != nil {
		if l, ok := ctx.Value(ctxKey{}).(*Logger); ok && l != nil {
			return l
		}
	}
	return Default()
}
