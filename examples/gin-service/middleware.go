package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/aidnetic/logger-go"
)

const (
	headerRequestID  = "X-Request-ID"
	headerLogLevel   = "X-Log-Level"   // per-request level, e.g. "debug"
	headerAdminToken = "X-Admin-Token" // required for X-Log-Level and /admin
)

// L returns the request-scoped logger. Handlers use L(c); code that only has
// a context.Context uses logger.FromContext(ctx) and gets the same logger.
func L(c *gin.Context) *logger.Logger { return logger.FromContext(c.Request.Context()) }

// RequestLogger creates a request-scoped logger and writes the access log.
//
// The request logger carries the request ID. Its level is, in order of priority:
//
//  1. the X-Log-Level header, if the request also has a valid admin token;
//  2. the route's level in levels, if one is set;
//  3. otherwise the base logger's level, so base.SetLevel still applies.
//
// The access log entry goes through the request logger at INFO for 1xx-3xx,
// WARN for 4xx and ERROR for 5xx. A route at WARN therefore logs only failed
// requests, and a route at OFF logs nothing.
func RequestLogger(base *logger.Logger, levels *RouteLevels, adminToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		id := c.GetHeader(headerRequestID)
		if !validRequestID(id) {
			id = newRequestID()
		}
		c.Header(headerRequestID, id)

		log := base.With(logger.F("request_id", id))
		if level, ok := levels.Get(routeKey(c)); ok {
			log = log.WithLevel(level)
		}
		if level, ok := requestLevel(c, adminToken); ok {
			log = log.WithLevel(level)
		}
		c.Request = c.Request.WithContext(logger.NewContext(c.Request.Context(), log))

		c.Next()

		status := c.Writer.Status()
		level := logger.INFO
		switch {
		case status >= 500:
			level = logger.ERROR
		case status >= 400:
			level = logger.WARN
		}
		if !log.Enabled(level) {
			return
		}
		fields := []logger.Field{
			slog.Int("status", status),
			slog.Duration("latency", time.Since(start)),
			slog.String("ip", c.ClientIP()),
			slog.Int("bytes", max(c.Writer.Size(), 0)),
		}
		if len(c.Errors) > 0 {
			fields = append(fields, slog.String("errors", strings.Join(c.Errors.Errors(), "; ")))
		}
		// LogAttrs: the access log runs on every request, so use the
		// allocation-free method.
		log.LogAttrs(level, c.Request.Method+" "+c.Request.URL.Path, fields...)
	}
}

// Recovery turns panics into 500 responses and logs them with the stack trace.
// The panic is logged even on routes whose level is OFF or WARN+.
func Recovery(base *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec) // net/http's signal to abort the response quietly
			}
			log := L(c)
			if !log.Enabled(logger.ERROR) {
				log = base.With(logger.F("request_id", c.Writer.Header().Get(headerRequestID))).WithLevel(logger.ERROR)
			}
			log.Errorf("panic: %v\n%s", rec, debug.Stack())
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}()
		c.Next()
	}
}

// requireToken rejects requests without the admin token.
func requireToken(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !validToken(c.GetHeader(headerAdminToken), token) {
			L(c).Warn("rejected admin request", logger.F("path", c.Request.URL.Path))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid " + headerAdminToken})
			return
		}
		c.Next()
	}
}

// routeKey identifies a route by method and gin pattern: "GET /api/users/:id".
// It is empty for requests that matched no route.
func routeKey(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return c.Request.Method + " " + p
	}
	return ""
}

// requestLevel returns the level from X-Log-Level, if the request is allowed
// to set one.
func requestLevel(c *gin.Context, adminToken string) (logger.Level, bool) {
	v := c.GetHeader(headerLogLevel)
	if v == "" || !validToken(c.GetHeader(headerAdminToken), adminToken) {
		return 0, false
	}
	level, err := logger.ParseLevel(v)
	return level, err == nil
}

func validToken(got, want string) bool {
	return want != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// validRequestID accepts client-supplied request IDs only if they are short
// and plain, so clients cannot flood the logs with arbitrary text.
func validRequestID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
