package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// DefaultTimeFormat is the console timestamp layout used when none is set.
const DefaultTimeFormat = "2006-01-02 15:04:05.000"

// timeOnlyMillis is the timestamp layout of the Development preset.
const timeOnlyMillis = "15:04:05.000"

// valueTimeFormat is the layout for time.Time field values.
const valueTimeFormat = "2006-01-02T15:04:05.000Z07:00"

// ConsoleOptions configures a ConsoleHandler created with NewConsoleHandler.
// Loggers created with New fill these in from Options.
type ConsoleOptions struct {
	// Level is the minimum level to write. Nil means INFO, as in the log/slog handlers.
	Level slog.Leveler

	// AddSource adds the caller's file and line as a source=dir/file.go:42 field.
	AddSource bool

	// Color turns ANSI colors on. NewConsoleHandler does not look at the
	// terminal; New does when Options.Color is ColorAuto.
	Color bool

	// TimeFormat is the timestamp layout. Empty means DefaultTimeFormat.
	TimeFormat string

	// NoTime leaves timestamps out.
	NoTime bool

	// UTC writes timestamps in UTC instead of local time.
	UTC bool

	// Theme sets the colors. Nil means DefaultTheme(). Ignored when Color is false.
	Theme *Theme

	// Layout controls level labels, alignment, truncation and multi-line fields.
	Layout Layout

	// ReplaceAttr rewrites or drops fields before they are written. Unlike the
	// log/slog handlers, it is not called for the time, level, message and source.
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
}

// ConsoleHandler is a slog.Handler that writes human-readable lines:
//
//	2026-11-14 10:44:34.001 ERROR  fetch user failed  error="dial tcp: i/o timeout" user_id=42
//
// Messages and values from untrusted input are escaped: control characters
// and ANSI escape sequences are written as \n, \x1b and so on, so they cannot
// forge log lines or change the terminal. Continuation lines of multi-line
// messages (such as stack traces) are indented under the message.
//
// A ConsoleHandler is safe for concurrent use. Handlers derived with WithAttrs
// and WithGroup share its lock, and every entry is written with a single Write
// call, so lines never interleave.
//
// Use it directly with log/slog:
//
//	slog.SetDefault(slog.New(logger.NewConsoleHandler(os.Stderr, &logger.ConsoleOptions{Color: true})))
type ConsoleHandler struct {
	c      *consoleConfig // shared by all derived handlers; never changed after creation
	attrs  [][]byte       // fields added with WithAttrs, already rendered
	prefix string         // key prefix from WithGroup, such as "http."
	groups []string       // group names from WithGroup, passed to ReplaceAttr
}

type consoleConfig struct {
	level      slog.Leveler
	addSource  bool
	noTime     bool
	utc        bool
	color      bool
	timeFormat string
	layout     Layout
	replace    func([]string, slog.Attr) slog.Attr
	theme      Theme
	fieldStyle func(string, slog.Value) Style

	// Precomputed styles; all empty when colors are off.
	sTime, sMsg, sKey, sVal, sErr, sSrc, reset string

	labels     [len(levelNames)]string // rendered labels for the named levels
	labelWidth int

	mu sync.Mutex
	w  io.Writer
}

// NewConsoleHandler returns a ConsoleHandler that writes to w.
func NewConsoleHandler(w io.Writer, opts *ConsoleOptions) *ConsoleHandler {
	if opts == nil {
		opts = &ConsoleOptions{}
	}
	c := &consoleConfig{
		level:      opts.Level,
		addSource:  opts.AddSource,
		noTime:     opts.NoTime,
		utc:        opts.UTC,
		color:      opts.Color,
		timeFormat: opts.TimeFormat,
		layout:     opts.Layout,
		replace:    opts.ReplaceAttr,
		w:          w,
	}
	if c.level == nil {
		c.level = INFO
	}
	if c.timeFormat == "" {
		c.timeFormat = DefaultTimeFormat
	}
	if c.color {
		if opts.Theme != nil {
			c.theme = *opts.Theme
		} else {
			c.theme = *DefaultTheme()
		}
		t := &c.theme
		c.sTime, c.sMsg, c.sKey = string(t.Time), string(t.Message), string(t.Key)
		c.sVal, c.sErr, c.sSrc = string(t.Value), string(t.ErrorValue), string(t.Source)
		c.reset = string(Reset)
		c.fieldStyle = t.FieldStyle
	}
	for _, n := range levelNames {
		if w := len(c.labelText(n.level)); w > c.labelWidth {
			c.labelWidth = w
		}
	}
	for i, n := range levelNames {
		c.labels[i] = string(c.renderLabel(nil, n.level))
	}
	return &ConsoleHandler{c: c}
}

// Enabled reports whether level is at or above the handler's minimum level.
func (h *ConsoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.c.level.Level()
}

// WithAttrs returns a handler that adds attrs to every entry. The attributes
// are rendered once, here, rather than on every entry.
func (h *ConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	h2 := h.clone()
	for _, a := range attrs {
		h.walk(h.prefix, h.groups, a, func(prefix, key string, v slog.Value) {
			h2.attrs = append(h2.attrs, h.c.appendField(nil, prefix, key, v))
		})
	}
	return h2
}

// WithGroup returns a handler that writes later fields as name.key=value.
func (h *ConsoleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := h.clone()
	h2.prefix = h.prefix + name + "."
	h2.groups = append(slices.Clip(h.groups), name)
	return h2
}

func (h *ConsoleHandler) clone() *ConsoleHandler {
	return &ConsoleHandler{c: h.c, attrs: slices.Clip(h.attrs), prefix: h.prefix, groups: h.groups}
}

// Handle writes one entry.
func (h *ConsoleHandler) Handle(_ context.Context, r slog.Record) error {
	c := h.c
	bp := bufPool.Get().(*[]byte)
	b := (*bp)[:0]

	// indent is the column where the message starts; continuation lines and
	// multi-line fields are indented to it.
	indent := 0
	if !c.noTime && !r.Time.IsZero() {
		t := r.Time
		if c.utc {
			t = t.UTC()
		}
		b = append(b, c.sTime...)
		start := len(b)
		b = appendTime(b, t, c.timeFormat)
		indent = len(b) - start + 1
		b = c.endStyle(b, c.sTime)
		b = append(b, ' ')
	}
	b = c.appendLevel(b, Level(r.Level))
	b = append(b, ' ')
	indent += c.labelWidth + 1

	first, rest, multiLine := strings.Cut(r.Message, "\n")
	if first != "" {
		b = append(b, c.sMsg...)
		b = appendEscaped(b, first)
		b = c.endStyle(b, c.sMsg)
	}

	if len(h.attrs) > 0 || r.NumAttrs() > 0 || (c.addSource && r.PC != 0) {
		fw := fieldWriter{c: c, b: b, indent: indent, first: true}
		if !c.layout.Multiline && first != "" {
			gap := 2
			if w := c.layout.MessageWidth; w > 0 {
				if n := utf8.RuneCountInString(first); n < w {
					gap += w - n
				}
			}
			fw.b = appendSpaces(fw.b, gap)
		}
		for _, f := range h.attrs {
			fw.writeRendered(f)
		}
		r.Attrs(func(a slog.Attr) bool {
			h.walk(h.prefix, h.groups, a, fw.writeField)
			return true
		})
		if c.addSource && r.PC != 0 {
			fw.writeSource(r.PC)
		}
		b = fw.b
	}

	// Continuation lines of the message come after the fields, so the fields
	// stay on the first line next to the message they describe.
	for multiLine {
		var line string
		line, rest, multiLine = strings.Cut(rest, "\n")
		b = append(b, '\n')
		b = appendSpaces(b, indent)
		b = append(b, c.sMsg...)
		b = appendEscaped(b, line)
		b = c.endStyle(b, c.sMsg)
	}
	b = append(b, '\n')

	c.mu.Lock()
	_, err := c.w.Write(b)
	c.mu.Unlock()

	if cap(b) <= maxPooledBuffer {
		*bp = b
		bufPool.Put(bp)
	}
	return err
}

// walk resolves a, flattens groups, applies ReplaceAttr and calls fn for every
// field that should be written.
func (h *ConsoleHandler) walk(prefix string, groups []string, a slog.Attr, fn func(prefix, key string, v slog.Value)) {
	a.Value = a.Value.Resolve()
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		if len(attrs) == 0 {
			return
		}
		if a.Key != "" {
			prefix = prefix + a.Key + "."
			groups = append(slices.Clip(groups), a.Key)
		}
		for _, ga := range attrs {
			h.walk(prefix, groups, ga, fn)
		}
		return
	}
	if a.Key == "" && a.Value.Kind() == slog.KindAny && a.Value.Any() == nil {
		return // empty field, such as Err(nil)
	}
	if h.c.replace != nil {
		a = h.c.replace(groups, a)
		a.Value = a.Value.Resolve()
		if a.Key == "" {
			return
		}
		if a.Value.Kind() == slog.KindGroup {
			for _, ga := range a.Value.Group() {
				h.walk(prefix+a.Key+".", append(slices.Clip(groups), a.Key), ga, fn)
			}
			return
		}
	}
	fn(prefix, a.Key, a.Value)
}

// fieldWriter appends fields to an entry with the right separators.
type fieldWriter struct {
	c      *consoleConfig
	b      []byte
	indent int
	first  bool
}

func (fw *fieldWriter) sep() {
	if fw.c.layout.Multiline {
		fw.b = append(fw.b, '\n')
		fw.b = appendSpaces(fw.b, fw.indent)
		return
	}
	if !fw.first {
		fw.b = append(fw.b, ' ')
	}
	fw.first = false
}

func (fw *fieldWriter) writeRendered(f []byte) {
	fw.sep()
	if fw.c.layout.Multiline && bytes.IndexByte(f, '\n') >= 0 {
		fw.b = appendIndented(fw.b, f, fw.indent)
		return
	}
	fw.b = append(fw.b, f...)
}

func (fw *fieldWriter) writeField(prefix, key string, v slog.Value) {
	fw.sep()
	start := len(fw.b)
	fw.b = fw.c.appendField(fw.b, prefix, key, v)
	// Pretty-printed values (Multiline only) contain newlines; indent them.
	if fw.c.layout.Multiline && bytes.IndexByte(fw.b[start:], '\n') >= 0 {
		f := slices.Clone(fw.b[start:])
		fw.b = appendIndented(fw.b[:start], f, fw.indent)
	}
}

func (fw *fieldWriter) writeSource(pc uintptr) {
	fw.sep()
	fw.b = append(fw.b, fw.c.sSrc...)
	fw.b = append(fw.b, "source="...)
	fw.b = append(fw.b, sourceLocation(pc)...)
	fw.b = fw.c.endStyle(fw.b, fw.c.sSrc)
}

func (c *consoleConfig) labelText(l Level) string {
	text := l.format(c.layout.LevelFormat == LevelShort)
	if c.color && c.theme.Badge {
		text = " " + text + " "
	}
	return text
}

func (c *consoleConfig) renderLabel(b []byte, l Level) []byte {
	text := c.labelText(l)
	style := ""
	if c.color {
		style = string(c.theme.levelStyle(l))
	}
	b = append(b, style...)
	b = append(b, text...)
	b = c.endStyle(b, style)
	return appendSpaces(b, c.labelWidth-len(text))
}

func (c *consoleConfig) appendLevel(b []byte, l Level) []byte {
	if i := levelIndex(l); i >= 0 {
		return append(b, c.labels[i]...)
	}
	return c.renderLabel(b, l)
}

func (c *consoleConfig) endStyle(b []byte, style string) []byte {
	if style == "" {
		return b
	}
	return append(b, c.reset...)
}

// appendField appends key=value with styles.
func (c *consoleConfig) appendField(b []byte, prefix, key string, v slog.Value) []byte {
	b = append(b, c.sKey...)
	if (prefix != "" && needsQuote(prefix)) || needsQuote(key) {
		b = strconv.AppendQuote(b, prefix+key)
	} else {
		b = append(b, prefix...)
		b = append(b, key...)
	}
	b = c.endStyle(b, c.sKey)
	b = append(b, '=')

	style := c.sVal
	if isError(v) {
		style = c.sErr
	}
	if c.fieldStyle != nil {
		if s := c.safeFieldStyle(prefix+key, v); s != "" {
			style = string(s)
		}
	}
	b = append(b, style...)
	start := len(b)
	b = c.appendValue(b, v)
	if max := c.layout.MaxValueLen; max > 0 {
		b = truncate(b, start, max)
	}
	return c.endStyle(b, style)
}

// safeFieldStyle calls Theme.FieldStyle, treating a panic (for example
// v.Int64() on a field that was logged as a string) as "no style".
func (c *consoleConfig) safeFieldStyle(key string, v slog.Value) (s Style) {
	defer func() {
		if recover() != nil {
			s = ""
		}
	}()
	return c.fieldStyle(key, v)
}

func isError(v slog.Value) bool {
	if v.Kind() != slog.KindAny {
		return false
	}
	_, ok := v.Any().(error)
	return ok
}

func (c *consoleConfig) appendValue(b []byte, v slog.Value) []byte {
	switch v.Kind() {
	case slog.KindString:
		return appendString(b, v.String())
	case slog.KindInt64:
		return strconv.AppendInt(b, v.Int64(), 10)
	case slog.KindUint64:
		return strconv.AppendUint(b, v.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.AppendFloat(b, v.Float64(), 'g', -1, 64)
	case slog.KindBool:
		return strconv.AppendBool(b, v.Bool())
	case slog.KindDuration:
		return append(b, v.Duration().String()...)
	case slog.KindTime:
		return v.Time().AppendFormat(b, valueTimeFormat)
	case slog.KindAny:
		return c.appendAny(b, v.Any())
	default:
		return appendString(b, v.String())
	}
}

// appendAny formats values of arbitrary types. It recovers from panics in
// String, Error and MarshalJSON methods, so a buggy value cannot crash the
// program that logs it.
func (c *consoleConfig) appendAny(b []byte, x any) (out []byte) {
	start := len(b)
	defer func() {
		if r := recover(); r != nil {
			out = append(b[:start], "<!PANIC: "...)
			out = appendEscaped(out, fmt.Sprint(r))
			out = append(out, '>')
		}
	}()
	switch v := x.(type) {
	case nil:
		return append(b, "<nil>"...)
	case error:
		return appendString(b, v.Error())
	case []byte:
		return appendString(b, string(v))
	case fmt.Stringer:
		return appendString(b, v.String())
	}
	if c.layout.Multiline && isComposite(x) {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(x); err == nil {
			// encoding/json escapes control characters in strings, so the
			// only raw newlines are its own line breaks.
			return append(b, bytes.TrimRight(buf.Bytes(), "\n")...)
		}
	}
	return appendEscaped(b, fmt.Sprintf("%+v", x))
}

func isComposite(x any) bool {
	t := reflect.TypeOf(x)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		return true
	}
	return false
}

// sourceCache maps program counters to "dir/file.go:line", so each call site
// is symbolized once.
var sourceCache sync.Map

func sourceLocation(pc uintptr) string {
	if s, ok := sourceCache.Load(pc); ok {
		return s.(string)
	}
	f, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	s := shortPath(f.File) + ":" + strconv.Itoa(f.Line)
	sourceCache.Store(pc, s)
	return s
}

// shortPath keeps the last directory and the file name.
func shortPath(p string) string {
	i := strings.LastIndexByte(p, '/')
	if i < 0 {
		return p
	}
	if j := strings.LastIndexByte(p[:i], '/'); j >= 0 {
		return p[j+1:]
	}
	return p
}

// needsQuote reports whether s must be quoted to be read back unambiguously:
// it is empty, or contains spaces, '=', '"', control or non-printable characters.
func needsQuote(s string) bool {
	if s == "" {
		return true
	}
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if c <= ' ' || c == '=' || c == '"' || c == 0x7f {
				return true
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError || !unicode.IsPrint(r) {
			return true
		}
		i += size
	}
	return false
}

func appendString(b []byte, s string) []byte {
	if needsQuote(s) {
		return strconv.AppendQuote(b, s)
	}
	return append(b, s...)
}

const hexDigits = "0123456789abcdef"

// appendEscaped appends s without quotes, escaping control characters,
// invalid UTF-8 and invisible formatting characters (such as bidirectional
// overrides) so that untrusted text cannot break lines or send escape
// sequences to the terminal. Tabs are kept.
func appendEscaped(b []byte, s string) []byte {
	for i := 0; i < len(s); {
		c := s[i]
		if c >= 0x20 && c < 0x7f || c == '\t' {
			b = append(b, c)
			i++
			continue
		}
		if c < utf8.RuneSelf {
			switch c {
			case '\n':
				b = append(b, `\n`...)
			case '\r':
				b = append(b, `\r`...)
			default:
				b = append(b, '\\', 'x', hexDigits[c>>4], hexDigits[c&0xf])
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			b = append(b, '\\', 'x', hexDigits[c>>4], hexDigits[c&0xf])
		case unicode.IsGraphic(r):
			b = append(b, s[i:i+size]...)
		default:
			b = append(b, `\u`...)
			b = append(b, fmt.Sprintf("%04x", r)...)
		}
		i += size
	}
	return b
}

// appendIndented appends src, indenting every line after the first.
func appendIndented(b, src []byte, indent int) []byte {
	for {
		i := bytes.IndexByte(src, '\n')
		if i < 0 {
			return append(b, src...)
		}
		b = append(b, src[:i+1]...)
		b = appendSpaces(b, indent)
		src = src[i+1:]
	}
}

// truncate cuts b[start:] to max runes, adding "…" if it was longer.
func truncate(b []byte, start, max int) []byte {
	v := b[start:]
	if utf8.RuneCount(v) <= max {
		return b
	}
	n, off := 0, 0
	for off < len(v) && n < max {
		_, size := utf8.DecodeRune(v[off:])
		off += size
		n++
	}
	return append(b[:start+off], "…"...)
}

// appendTime formats t with layout. The default layouts are formatted by hand,
// which is several times faster than time.Time.AppendFormat.
func appendTime(b []byte, t time.Time, layout string) []byte {
	switch layout {
	case DefaultTimeFormat:
		y, m, d := t.Date()
		if y < 0 || y > 9999 {
			break
		}
		b = append(b, byte('0'+y/1000), byte('0'+y/100%10), byte('0'+y/10%10), byte('0'+y%10), '-')
		b = append2(b, int(m))
		b = append(b, '-')
		b = append2(b, d)
		b = append(b, ' ')
		return appendClock(b, t)
	case timeOnlyMillis:
		return appendClock(b, t)
	}
	return t.AppendFormat(b, layout)
}

// appendClock appends t as 15:04:05.000.
func appendClock(b []byte, t time.Time) []byte {
	h, m, s := t.Clock()
	ms := t.Nanosecond() / 1e6
	b = append2(b, h)
	b = append(b, ':')
	b = append2(b, m)
	b = append(b, ':')
	b = append2(b, s)
	return append(b, '.', byte('0'+ms/100), byte('0'+ms/10%10), byte('0'+ms%10))
}

func append2(b []byte, n int) []byte { return append(b, byte('0'+n/10), byte('0'+n%10)) }

const spaces = "                                                                "

func appendSpaces(b []byte, n int) []byte {
	for n > 0 {
		k := min(n, len(spaces))
		b = append(b, spaces[:k]...)
		n -= k
	}
	return b
}

// maxPooledBuffer keeps unusually large entries from pinning memory in the pool.
const maxPooledBuffer = 64 << 10

var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 1024)
		return &b
	},
}

var _ slog.Handler = (*ConsoleHandler)(nil)
