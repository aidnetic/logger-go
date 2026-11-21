# Examples

Runnable programs that use the logger. This directory is its own Go module,
so gin and the other example dependencies never become dependencies of the
logger itself. It builds against the logger in the parent directory.

```sh
cd examples
go run ./basic        # print style, printf style, levels, multi-line messages
go run ./fields       # F, Err, Group, With, structs, redaction, multi-line layout, LogAttrs
go run ./production   # JSON, FromEnv, redaction, OnError, SetDefault
go run ./themes       # themes, short level names, alignment, truncation, custom styles
go run ./stdlib       # log/slog and the std log package, Slog(), StdLogger(), plain slog
go test ./...         # includes the gin service tests
```

`production` reads `LOG_LEVEL`, `LOG_FORMAT`, `LOG_COLOR` and `LOG_SOURCE`:

```sh
LOG_LEVEL=debug LOG_FORMAT=console go run ./production
LOG_LEVEL=loud go run ./production   # fails at startup and names the bad value
```

## gin-service

A small HTTP API showing the pattern for services:

| File | What it shows |
|---|---|
| `main.go` | logger setup for development and production, `SetDefault`, `http.Server.ErrorLog`, graceful shutdown |
| `middleware.go` | the request-scoped logger, access logs, panic recovery, per-request level override |
| `levels.go` | per-route log levels with lock-free reads |
| `admin.go` | endpoints to inspect and change levels at run time, with audit logs |
| `handlers.go` | handlers and repository code using the request logger |
| `main_test.go` | tests for all of the above with `logtest` |

### How per-API logging works

Each request gets its own logger, created by `RequestLogger`:

```go
log := base.With(logger.F("request_id", id))
if level, ok := levels.Get("GET /api/users/:id"); ok { // the matched route
	log = log.WithLevel(level)
}
ctx = logger.NewContext(ctx, log)
```

Handlers call `L(c)`. Code that only has a `context.Context` calls
`logger.FromContext(ctx)`. Both get the same logger, so every line of a
request carries its request ID and follows its route's level:

| Route level | What that route logs |
|---|---|
| not set | follows the service's level (changing it changes the route too) |
| `debug` / `trace` | everything, for that route only |
| `warn` | only failures: 4xx/5xx access logs, warnings and errors |
| `off` | nothing, except panics, which are always logged |

The access log entry is written at INFO for 1xx–3xx, WARN for 4xx and ERROR
for 5xx. That is what makes `warn` mean "only failed requests".

### Try it

```sh
ADMIN_TOKEN=secret go run ./gin-service
```

In another terminal:

```sh
B=http://localhost:8080
T='X-Admin-Token: secret'

curl $B/healthz                 # never logged (the route is set to off)
curl $B/api/users/1             # 200: access log at INFO
curl $B/api/users/99            # 404: access log at WARN
curl $B/api/users/13            # 500: the error and the access log at ERROR
curl $B/api/panic               # panic: stack trace at ERROR, 500 response

# See the levels.
curl -H "$T" $B/admin/log-levels

# Trace one route only; the repository code's TRACE lines appear.
curl -X PUT -H "$T" $B/admin/log-levels/routes -d '{"route":"GET /api/users/:id","level":"trace"}'
curl $B/api/users/2

# Log only failures for that route, then remove the override.
curl -X PUT -H "$T" $B/admin/log-levels/routes -d '{"route":"GET /api/users/:id","level":"warn"}'
curl -X DELETE -H "$T" "$B/admin/log-levels/routes?route=GET%20/api/users/:id"

# Change the service's level.
curl -X PUT -H "$T" $B/admin/log-levels/default -d '{"level":"warn"}'

# Debug one request without changing any level (needs the admin token).
curl -H 'X-Log-Level: debug' -H "$T" -X POST $B/api/orders \
     -d '{"item":"book","quantity":2,"card_number":"4242424242424242"}'   # card_number is redacted
```

In production mode the same logs come out as JSON:

```sh
APP_ENV=production ADMIN_TOKEN=secret go run ./gin-service
```

### Notes for real services

- The admin endpoints are protected by a shared token, compared in constant
  time. In a real deployment, put them behind your normal authentication and
  keep them off the public listener.
- Route levels live in memory, per instance. To change them across a fleet,
  load them from your config system and call `levels.Set` when it changes.
- Client-supplied `X-Request-ID` values are accepted only if they are short
  and plain; anything else is replaced.
- `SetTrustedProxies(nil)` stops clients from spoofing their IP with
  `X-Forwarded-For`. Behind a load balancer, list its addresses instead.
