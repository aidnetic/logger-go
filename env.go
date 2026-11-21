package logger

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Environment variables read by FromEnv.
const (
	EnvLevel  = "LOG_LEVEL"  // trace, debug, info, notice, warn, error, fatal, off
	EnvFormat = "LOG_FORMAT" // console, json, text
	EnvColor  = "LOG_COLOR"  // auto, always, never
	EnvSource = "LOG_SOURCE" // true or false: add caller file:line
)

// FromEnv returns base with any settings from the LOG_LEVEL, LOG_FORMAT,
// LOG_COLOR and LOG_SOURCE environment variables applied. Unset or empty
// variables leave base unchanged.
//
// It returns an error listing every invalid value, so a typo fails at startup
// instead of silently logging at the wrong level:
//
//	opts, err := logger.FromEnv(logger.Production())
//	if err != nil {
//		logger.Fatal("invalid logging configuration:", err)
//	}
//	log := logger.New(opts)
//
// The package never reads the environment on its own; only FromEnv does.
func FromEnv(base Options) (Options, error) {
	var errs []error
	if v := env(EnvLevel); v != "" {
		if l, err := ParseLevel(v); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", EnvLevel, err))
		} else {
			base.Level = l
		}
	}
	if v := env(EnvFormat); v != "" {
		if f, err := ParseFormat(v); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", EnvFormat, err))
		} else {
			base.Format = f
		}
	}
	if v := env(EnvColor); v != "" {
		if c, err := ParseColorMode(v); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", EnvColor, err))
		} else {
			base.Color = c
		}
	}
	if v := env(EnvSource); v != "" {
		if b, err := strconv.ParseBool(v); err != nil {
			errs = append(errs, fmt.Errorf("%s: invalid boolean %q", EnvSource, v))
		} else {
			base.AddSource = b
		}
	}
	return base, errors.Join(errs...)
}

func env(key string) string { return strings.TrimSpace(os.Getenv(key)) }
