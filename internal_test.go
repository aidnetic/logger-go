package logger

import (
	"math/rand"
	"testing"
	"time"
)

func TestAppendTimeMatchesAppendFormat(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	zones := []*time.Location{time.UTC, time.FixedZone("x", -7*3600), time.FixedZone("y", 5*3600+1800)}
	for i := 0; i < 5000; i++ {
		tm := time.Unix(rng.Int63n(1<<35), rng.Int63n(1e9)).In(zones[i%len(zones)])
		for _, layout := range []string{DefaultTimeFormat, timeOnlyMillis, time.RFC3339} {
			got := string(appendTime(nil, tm, layout))
			if want := tm.Format(layout); got != want {
				t.Fatalf("appendTime(%v, %q) = %q, want %q", tm, layout, got, want)
			}
		}
	}
	// Years outside 0..9999 fall back to time.Format.
	far := time.Date(12345, 1, 2, 3, 4, 5, 6e6, time.UTC)
	if got, want := string(appendTime(nil, far, DefaultTimeFormat)), far.Format(DefaultTimeFormat); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestShortPath(t *testing.T) {
	for in, want := range map[string]string{
		"/home/u/app/handlers/user.go": "handlers/user.go",
		"user.go":                      "user.go",
		"/user.go":                     "/user.go",
		"a/user.go":                    "a/user.go",
	} {
		if got := shortPath(in); got != want {
			t.Errorf("shortPath(%q) = %q, want %q", in, got, want)
		}
	}
}
