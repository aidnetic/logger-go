# logger

Leveled, structured logging for Go services, with print-style methods and
console output that people can actually read. Built on `log/slog`; depends
only on the standard library.

```
10:44:33.402 INFO   server listening on :8080
10:44:33.950 NOTICE user signed up  user_id=42 plan=pro
10:44:34.001 ERROR  payment failed  request_id=7f3a error="card declined" order_id=8812
10:44:34.120 ERROR  panic: index out of range  request_id=9c1e
                    goroutine 41 [running]:
                    main.(*Cart).Total(...)
```

The same calls in production, as JSON:

```json
{"time":"2026-11-14T10:44:34.001Z","level":"ERROR","msg":"payment failed","request_id":"7f3a","error":"card declined","order_id":8812}
```

- **Print style.** `log.Info("listening on", addr)` and `log.Infof("took %v", d)`, checked by `go vet`.
- **Structured fields** when you want them: `log.Error("payment failed", logger.Err(err), logger.F("order_id", id))`.
- **Levels:** TRACE, DEBUG, INFO, NOTICE, WARN, ERROR, FATAL and OFF. Change them at run time, or per request, route or component.
- **Readable console output:** aligned levels, colors, themes, multi-line layout, and safe escaping of untrusted input.
- **JSON and logfmt** for log pipelines.
- **Works with the ecosystem:** any `slog.Handler` as output, and `*slog.Logger` / `*log.Logger` for libraries.
- **Built for production:** safe for concurrent use, one write per entry, configuration that fails fast, redaction, 0-allocation hot path.

## Install

```sh
go get github.com/aidnetic/logger-go
```

Requires Go 1.22 or later.

## Quick start

```go
package main

import (
	"errors"

	"github.com/aidnetic/logger-go"
)

func main() {
	opts, err := logger.FromEnv(logger.Development()) // LOG_LEVEL, LOG_FORMAT, ...
	if err != nil {
		logger.Fatal("invalid logging configuration:", err)
	}
	log := logger.New(opts)
	logger.SetDefault(log) // log/slog and the standard log package now use it too

	log.Info("server listening on", ":8080")
	log.Infof("loaded %d routes", 12)
	log.Error("payment failed", logger.Err(errors.New("card declined")), logger.F("order_id", 8812))
}
```

## Logging

### Print style and printf style

Every level has a print-style method, which joins its arguments like
`fmt.Println`, and an `f` method, which formats like `fmt.Printf`:

```go
log.Info("user", id, "logged in from", ip)    // INFO   user 42 logged in from 10.0.0.1
log.Infof("user %d logged in from %s", id, ip) // same output
```

`go vet` checks both, so `log.Info("count %d", n)` and `log.Infof("%d", "x")`
are reported at build time.

### Fields

Pass `Field` values anywhere in the arguments. They are taken out of the
message and written as key/value data:

```go
log.Info("user signed up", logger.F("user_id", 42), logger.F("plan", "pro"))
// console: INFO   user signed up  user_id=42 plan=pro
// JSON:    {"level":"INFO","msg":"user signed up","user_id":42,"plan":"pro"}
```

| Constructor | Writes |
|---|---|
| `logger.F(key, value)` | any value: numbers, strings, durations, structs, maps |
| `logger.Err(err)` | `error="..."`; skipped when `err` is nil |
| `logger.Group(key, fields...)` | nested fields: `http.method=GET http.status=200` |
| `slog.Int`, `slog.String`, `slog.Duration`, … | `Field` is an alias for `slog.Attr`, so these work too |

Structs, maps and slices are written readably (`user={ID:42 Name:Ada}`), and
pretty-printed as indented JSON in the multi-line layout.

### Levels

| Level | Use for |
|---|---|
| `TRACE` | very detailed diagnostics, usually off even in development |
| `DEBUG` | diagnostics for developers |
| `INFO` | normal operation (the default) |
| `NOTICE` | significant but expected events: user signed up, payment captured |
| `WARN` | something unexpected that the program recovered from |
| `ERROR` | a failed operation that needs attention |
| `FATAL` | the process cannot continue |
| `OFF` | a threshold that disables logging |

The values match `log/slog` (DEBUG = −4, INFO = 0, WARN = 4, ERROR = 8), so a
`Level` converts to `slog.Level` with a plain conversion. `Level` implements
`encoding.TextUnmarshaler` and `flag.Value`, so it can be read from config
files and flags. `ParseLevel` ignores case and returns an error for unknown
names instead of guessing.

```go
log.SetLevel(logger.DEBUG)       // safe while other goroutines log
if log.Enabled(logger.DEBUG) {   // skip expensive arguments when the level is off
	log.Debug("state:", dump())
}
log.Log(level, "decided at run time") // never exits, even at FATAL
```

`Fatal` and `Fatalf` log (whatever the level), flush the output if it has a
`Sync` method, and exit with status 1. Deferred functions do not run. Tests
can set `Options.ExitFunc`.

### Hot paths

Go boxes every argument passed through `...any`, so each `Field` given to a
print-style method costs one small allocation. That is fine almost
everywhere. For code that logs on every request or message, `LogAttrs` takes
fields without boxing them and does not allocate:

```go
log.LogAttrs(logger.INFO, "request handled", slog.Int("status", 200), slog.Duration("latency", d))
```

## Request-scoped loggers

`With` returns a Logger that adds fields to every entry. `WithLevel` gives a
Logger its own level. Both are cheap and share the parent's output and lock.

```go
reqLog := log.With(logger.F("request_id", id), logger.F("user_id", uid))
if debugRequested {
	reqLog = reqLog.WithLevel(logger.DEBUG) // this request only
}
```

Store it in the request context and get it back anywhere with `FromContext`.
It returns `Default()` when the context has none, so it never returns nil.

```go
ctx = logger.NewContext(ctx, reqLog)
// ... in a repository three calls down:
logger.FromContext(ctx).Debug("cache miss for", key)
```

A Logger created with `With` shares its parent's level: `SetLevel` on the
root still affects it. A Logger created with `WithLevel` has its own.
`WithLevel(logger.OFF)` silences a Logger.

[examples/gin-service](examples/gin-service) shows the full pattern: a
request-scoped logger per request, a level per API route that can be changed
at run time, debug logging for one request via a header, and access logs.

## Configuration

`logger.New(opts)` takes an `Options` struct. The zero value works: INFO,
console format, standard output, colors when it is a terminal.

| Field | Default | |
|---|---|---|
| `Level` | `INFO` | minimum level |
| `Format` | `FormatConsole` | `FormatConsole`, `FormatJSON`, or `FormatText` (logfmt) |
| `Output` | `os.Stdout` | any `io.Writer`; it does not need to be safe for concurrent use |
| `Handler` | | any `slog.Handler`; replaces the built-in output entirely |
| `TimeFormat` | `2006-01-02 15:04:05.000` (console), RFC 3339 (JSON) | timestamp layout |
| `NoTime` | `false` | leave timestamps out (when the platform adds its own) |
| `UTC` | `false` | timestamps in UTC |
| `AddSource` | `false` | add the caller's `file:line` |
| `Color` | `ColorAuto` | `ColorAuto`, `ColorAlways`, `ColorNever` |
| `Theme` | `DefaultTheme()` | console colors |
| `Layout` | inline | console layout: level names, alignment, truncation, multi-line |
| `ReplaceAttr` | | rewrite or drop fields; see `RedactKeys` |
| `OnError` | ignore | called when an entry cannot be written |
| `ExitFunc` | `os.Exit` | called by `Fatal` |

### Presets

| | `Development()` | `Production()` |
|---|---|---|
| Level | DEBUG | INFO |
| Format | console, colored on terminals | JSON |
| Timestamps | `15:04:05.000`, local time | RFC 3339, UTC |
| Caller | yes | no |

Presets are plain `Options` values, so you can change any field before calling `New`.

### Environment variables

The package never reads the environment by itself. `FromEnv(base)` applies
these on top of `base`, and returns an error naming every invalid value so a
typo fails at startup:

| Variable | Values |
|---|---|
| `LOG_LEVEL` | `trace`, `debug`, `info`, `notice`, `warn`, `error`, `fatal`, `off` |
| `LOG_FORMAT` | `console`, `json`, `text` |
| `LOG_COLOR` | `auto`, `always`, `never` |
| `LOG_SOURCE` | `true`, `false` |

## Console output

### Themes and layout

```go
opts.Theme = logger.BadgeTheme() // or DefaultTheme(), MinimalTheme()
opts.Layout = logger.Layout{
	LevelFormat:  logger.LevelShort, // INF, WRN, ERR
	MessageWidth: 32,                // line fields up in a column
	MaxValueLen:  200,               // cut long values with "…"
	Multiline:    true,              // one field per line, structs as indented JSON
}
```

Customize a theme by changing its fields. `Style` values are ANSI sequences
that combine with `+`, and `RGB` and `Color256` give 24-bit and 256-color styles:

```go
theme := logger.DefaultTheme()
theme.Info = logger.Bold + logger.RGB(80, 200, 120)
theme.FieldStyle = func(key string, v slog.Value) logger.Style {
	if key == "status" && v.Kind() == slog.KindInt64 && v.Int64() >= 500 {
		return logger.Bold + logger.Red
	}
	return ""
}
```

`go run ./themes` in [examples](examples) shows them all.

### Colors

`ColorAuto` uses colors only when the output is a terminal, and honors
[`NO_COLOR`](https://no-color.org), `FORCE_COLOR` and `TERM=dumb`. Legacy
Windows consoles that do not understand ANSI sequences need `ColorNever`;
Windows Terminal and modern PowerShell are fine.

### Untrusted input

Messages and values often contain user input: paths, headers, form fields. The
console format escapes control characters, ANSI escape sequences, invalid
UTF-8 and invisible formatting characters (such as bidirectional overrides),
so input cannot forge log lines or change what the terminal shows. String
values are quoted when needed (`header="a\nb"`). Continuation lines of
multi-line messages are indented under the message, so they can never be
mistaken for a new entry. The JSON format escapes by definition.

A value whose `String`, `Error` or `MarshalJSON` method panics is written as
`<!PANIC: ...>` instead of crashing the program.

## Production checklist

- Use `Production()` (JSON, UTC) and `FromEnv`, and exit if `FromEnv` returns an error.
- Call `logger.SetDefault(log)` once at startup, so libraries using `log` or
  `log/slog` end up in the same stream.
- Create one Logger per process and derive the rest with `With` and
  `WithLevel`. Two `New` calls with the same non-thread-safe writer have
  separate locks.
- Pass secrets as fields, not in the message, and set
  `ReplaceAttr: logger.RedactKeys("password", "token", "authorization", ...)`.
  Better still, implement `slog.LogValuer` on types that hold secrets.
- Set `OnError` to count write failures in your metrics.
- Use `LogAttrs` on per-request and per-message hot paths.
- `AddSource` adds about 270 ns to every entry (a stack lookup; the
  file:line text is cached per call site, so it does not allocate). Turn it
  on where you need it.
- `Fatal` does not run deferred functions. In long-running services, prefer
  returning errors to `main` and logging there.

## Integrations

Output goes through a `slog.Handler`, so anything that speaks `slog` works.
To send entries to OpenTelemetry, or to several destinations at once
(`slog.NewMultiHandler` needs Go 1.26+):

```go
console := logger.NewConsoleHandler(os.Stdout, &logger.ConsoleOptions{Level: slog.LevelDebug, Color: true})
log := logger.New(logger.Options{
	Level:   logger.DEBUG,
	Handler: slog.NewMultiHandler(console, otelslog.NewHandler("orders-api")),
})
```

In the other direction:

| | |
|---|---|
| `log.Slog()` | a `*slog.Logger` for libraries; keeps the fields and the level |
| `log.StdLogger(level)` | a `*log.Logger`, e.g. for `http.Server.ErrorLog` |
| `logger.SetDefault(log)` | routes `slog.Info` and `log.Printf` through `log` |
| `logger.NewConsoleHandler` | the readable console format for plain `log/slog` users |

## Testing

`logtest` records entries in memory:

```go
log, rec := logtest.New()
svc := NewCheckout(log)
svc.Pay(ctx, order)

e, ok := rec.Find("payment failed")
if !ok || e.Fields["order_id"] != order.ID {
	t.Fatalf("missing payment failure log: %v", rec.Entries())
}
```

`Fatal` records the exit code (`rec.ExitCodes()`) instead of exiting.

## Performance

`go test -bench . ` on an Intel i7-9850H, writing to a no-op writer:

| Entry | ns/op | allocs |
|---|---:|---:|
| `Info("request handled")` | 180 | 0 |
| `Info("user", 42, "logged in from", ip)` | 620 | 1 |
| `Infof("user %d logged in from %s", 42, ip)` | 620 | 1 |
| `LogAttrs(INFO, msg, 3 fields)` | 500 | 0 |
| `Info(msg, 3 fields)` | 700–2,300 † | 3 |
| child Logger with 2 fields, `Info(msg, 1 field)` | 1,000 | 1 |
| level disabled | 4 | 0 |
| *slog `TextHandler`, 3 fields (baseline)* | 940 | 0 |
| *slog `JSONHandler`, 3 fields (baseline)* | 830 | 0 |

† The three allocations come from boxing the fields into `...any`. In this
microbenchmark the heap is tiny, so the garbage collector runs constantly and
its fixed cost lands on every call (2.3 µs); with the collector off the same
call takes 0.7 µs. Services with normal heaps are in between. Use `LogAttrs`
where it matters.

`TestAllocations` pins these allocation counts, so a regression fails the tests.

## Examples

[examples](examples) is a separate module (so the logger itself has no
dependencies) with runnable programs: `basic`, `fields`, `production`,
`themes`, `stdlib`, and `gin-service`.

## Development

```sh
go vet ./... && go test ./...      # the logger
go test -race ./...                # needs cgo (a C compiler)
go test -run '^$' -bench . ./...   # benchmarks
cd examples && go test ./...       # examples, including the gin service tests
```
