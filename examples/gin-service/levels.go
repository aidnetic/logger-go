package main

import (
	"maps"
	"sync"
	"sync/atomic"

	"github.com/aidnetic/logger-go"
)

// RouteLevels holds a log level per route. A route is the HTTP method and
// the gin route pattern, such as "GET /api/users/:id". Routes without an
// entry use the base logger's level.
//
// Every request reads it and only operators write it, so reads are lock-free:
// a write copies the map and swaps it in.
type RouteLevels struct {
	mu     sync.Mutex // serializes writers
	levels atomic.Pointer[map[string]logger.Level]
}

// NewRouteLevels returns an empty RouteLevels.
func NewRouteLevels() *RouteLevels {
	rl := &RouteLevels{}
	m := map[string]logger.Level{}
	rl.levels.Store(&m)
	return rl
}

// Get returns the level set for route, if any.
func (rl *RouteLevels) Get(route string) (logger.Level, bool) {
	if route == "" {
		return 0, false
	}
	l, ok := (*rl.levels.Load())[route]
	return l, ok
}

// Set sets the level for route. logger.OFF silences the route.
func (rl *RouteLevels) Set(route string, level logger.Level) {
	rl.update(func(m map[string]logger.Level) { m[route] = level })
}

// Reset removes the route's level, so it follows the base logger again.
func (rl *RouteLevels) Reset(route string) {
	rl.update(func(m map[string]logger.Level) { delete(m, route) })
}

// All returns a copy of every route's level.
func (rl *RouteLevels) All() map[string]logger.Level {
	return maps.Clone(*rl.levels.Load())
}

func (rl *RouteLevels) update(fn func(map[string]logger.Level)) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	m := maps.Clone(*rl.levels.Load())
	fn(m)
	rl.levels.Store(&m)
}
