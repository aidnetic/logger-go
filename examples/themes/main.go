// Command themes shows the console themes and layout options. Colors are
// forced on so the output stays colored when piped.
//
//	go run ./themes
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aidnetic/logger-go"
)

func main() {
	show("Default theme", logger.Options{})
	show("Badge theme", logger.Options{Theme: logger.BadgeTheme()})
	show("Minimal theme, short level names", logger.Options{
		Theme:  logger.MinimalTheme(),
		Layout: logger.Layout{LevelFormat: logger.LevelShort},
	})
	show("Fields aligned in a column", logger.Options{Layout: logger.Layout{MessageWidth: 18}})

	// A custom theme: 24-bit colors, and HTTP status codes colored by class.
	theme := logger.DefaultTheme()
	theme.Info = logger.Bold + logger.RGB(80, 200, 120)
	theme.Key = logger.Color256(244)
	theme.FieldStyle = func(key string, v slog.Value) logger.Style {
		if key != "status" || v.Kind() != slog.KindInt64 {
			return ""
		}
		switch code := v.Int64(); {
		case code >= 500:
			return logger.Bold + logger.Red
		case code >= 400:
			return logger.Yellow
		default:
			return logger.Green
		}
	}
	fmt.Println("\nCustom theme with per-field styles")
	log := logger.New(logger.Options{Color: logger.ColorAlways, Theme: theme, TimeFormat: "15:04:05"})
	log.Info("GET /api/users/42", logger.F("status", 200), logger.F("latency", 2*time.Millisecond))
	log.Info("GET /api/users/99", logger.F("status", 404), logger.F("latency", time.Millisecond))
	log.Info("POST /api/orders", logger.F("status", 503), logger.F("latency", 30*time.Second))

	fmt.Println("\nLong values cut at 24 characters")
	logger.New(logger.Options{Color: logger.ColorAlways, TimeFormat: "15:04:05", Layout: logger.Layout{MaxValueLen: 24}}).
		Info("webhook received", logger.F("body", `{"event":"invoice.paid","data":{`+strings.Repeat(`"x":1,`, 20)+`}}`))

	fmt.Println("\nMulti-line fields")
	logger.New(logger.Options{Color: logger.ColorAlways, TimeFormat: "15:04:05", Layout: logger.Layout{Multiline: true}}).
		Info("config loaded", logger.F("database", map[string]any{"host": "db.internal", "pool": 20}), logger.F("env", "staging"))
}

func show(title string, opts logger.Options) {
	fmt.Println("\n" + title)
	opts.Level = logger.TRACE
	opts.Color = logger.ColorAlways
	opts.TimeFormat = "15:04:05"
	log := logger.New(opts)
	log.Trace("entering handler")
	log.Debug("cache warmed", logger.F("entries", 1200))
	log.Info("server listening", logger.F("addr", ":8080"))
	log.Notice("user signed up", logger.F("user_id", 42))
	log.Warn("slow query", logger.F("took", 1200*time.Millisecond))
	log.Error("payment failed", logger.Err(errors.New("card declined")))
	log.Log(logger.FATAL, "would exit here (Log never exits)")
}
