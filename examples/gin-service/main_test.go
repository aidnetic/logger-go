package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/aidnetic/logger-go"
	"github.com/aidnetic/logger-go/logtest"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

type testServer struct {
	router *gin.Engine
	log    *logger.Logger
	rec    *logtest.Recorder
	levels *RouteLevels
}

const token = "secret"

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	log, rec := logtest.New()
	log.SetLevel(logger.INFO)
	levels := NewRouteLevels()
	levels.Set("GET /healthz", logger.OFF)
	return &testServer{router: newRouter(log, levels, token), log: log, rec: rec, levels: levels}
}

// do sends a request; headers are name/value pairs.
func (s *testServer) do(method, path, body string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

func (s *testServer) messages() []string {
	var out []string
	for _, e := range s.rec.Entries() {
		out = append(out, e.Level.String()+" "+e.Message)
	}
	return out
}

func TestHealthzIsNeverLogged(t *testing.T) {
	s := newTestServer(t)
	if w := s.do("GET", "/healthz", ""); w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if s.rec.Len() != 0 {
		t.Fatalf("health check was logged: %q", s.messages())
	}
}

func TestAccessLog(t *testing.T) {
	s := newTestServer(t)
	w := s.do("GET", "/api/users/1", "", headerRequestID, "req-123")
	if w.Code != http.StatusOK || w.Header().Get(headerRequestID) != "req-123" {
		t.Fatalf("status %d, request id %q", w.Code, w.Header().Get(headerRequestID))
	}
	entries := s.rec.Entries()
	if len(entries) != 1 {
		t.Fatalf("want only the access log at INFO, got %q", s.messages())
	}
	e := entries[0]
	if e.Message != "GET /api/users/1" || e.Level != logger.INFO || e.Fields["status"] != int64(200) || e.Fields["request_id"] != "req-123" {
		t.Errorf("access log = %v", e)
	}
	if _, ok := e.Fields["latency"]; !ok {
		t.Errorf("access log has no latency: %v", e)
	}
}

func TestAccessLogLevelFollowsStatus(t *testing.T) {
	s := newTestServer(t)
	s.do("GET", "/api/users/999", "")
	s.do("GET", "/api/users/13", "")
	want := []string{
		"INFO user not found",
		"WARN GET /api/users/999",
		"ERROR user lookup failed",
		"ERROR GET /api/users/13",
	}
	if got := s.messages(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestRouteLevelTraceAppliesToThatRouteOnly(t *testing.T) {
	s := newTestServer(t)
	s.levels.Set("GET /api/users/:id", logger.TRACE)

	s.do("GET", "/api/users/1", "", headerRequestID, "r1")
	want := []string{
		"DEBUG looking up user",
		"TRACE querying users table", // from code that only has the context
		"DEBUG user loaded",
		"INFO GET /api/users/1",
	}
	if got := s.messages(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	for _, e := range s.rec.Entries() {
		if e.Fields["request_id"] != "r1" {
			t.Errorf("entry without the request id: %v", e)
		}
	}

	s.rec.Reset()
	s.do("POST", "/api/orders", `{"item":"book","quantity":1,"card_number":"4242"}`)
	if _, ok := s.rec.Find("order request"); ok {
		t.Error("DEBUG leaked into a route that still uses the default level")
	}
}

func TestRouteLevelWarnLogsOnlyFailures(t *testing.T) {
	s := newTestServer(t)
	s.levels.Set("GET /api/users/:id", logger.WARN)
	s.do("GET", "/api/users/1", "")
	if s.rec.Len() != 0 {
		t.Fatalf("successful request was logged: %q", s.messages())
	}
	s.do("GET", "/api/users/13", "")
	if got := s.messages(); len(got) != 2 {
		t.Errorf("failed request: got %q", got)
	}
}

func TestPerRequestLevelNeedsAdminToken(t *testing.T) {
	s := newTestServer(t)
	s.do("GET", "/api/users/1", "", headerLogLevel, "debug")
	if _, ok := s.rec.Find("looking up user"); ok {
		t.Fatal("X-Log-Level without a token was honored")
	}
	s.do("GET", "/api/users/1", "", headerLogLevel, "debug", headerAdminToken, "wrong")
	if _, ok := s.rec.Find("looking up user"); ok {
		t.Fatal("X-Log-Level with a wrong token was honored")
	}
	s.do("GET", "/api/users/1", "", headerLogLevel, "debug", headerAdminToken, token)
	if _, ok := s.rec.Find("looking up user"); !ok {
		t.Fatalf("X-Log-Level with the token was ignored: %q", s.messages())
	}
}

func TestAdminEndpoints(t *testing.T) {
	s := newTestServer(t)
	setRoute := `{"route":"GET /api/users/:id","level":"debug"}`

	if w := s.do("PUT", "/admin/log-levels/routes", setRoute); w.Code != http.StatusUnauthorized {
		t.Fatalf("without token: status %d", w.Code)
	}
	if w := s.do("PUT", "/admin/log-levels/routes", `{"route":"GET /nope","level":"debug"}`, headerAdminToken, token); w.Code != http.StatusNotFound {
		t.Fatalf("unknown route: status %d", w.Code)
	}
	if w := s.do("PUT", "/admin/log-levels/routes", `{"route":"GET /api/users/:id","level":"loud"}`, headerAdminToken, token); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid level: status %d", w.Code)
	}
	if w := s.do("PUT", "/admin/log-levels/routes", setRoute, headerAdminToken, token); w.Code != http.StatusOK {
		t.Fatalf("set route level: status %d %s", w.Code, w.Body)
	}
	if e, ok := s.rec.Find("route log level changed"); !ok || e.Level != logger.NOTICE {
		t.Errorf("admin change was not audited: %q", s.messages())
	}

	s.rec.Reset()
	s.do("GET", "/api/users/1", "")
	if _, ok := s.rec.Find("looking up user"); !ok {
		t.Fatalf("route level not applied: %q", s.messages())
	}

	w := s.do("GET", "/admin/log-levels", "", headerAdminToken, token)
	var got struct {
		Default string            `json:"default"`
		Routes  map[string]string `json:"routes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Default != "INFO" || got.Routes["GET /api/users/:id"] != "DEBUG" || got.Routes["GET /healthz"] != "OFF" {
		t.Errorf("GET /admin/log-levels = %s", w.Body)
	}

	if w := s.do("DELETE", "/admin/log-levels/routes?route="+url.QueryEscape("GET /api/users/:id"), "", headerAdminToken, token); w.Code != http.StatusNoContent {
		t.Fatalf("reset: status %d", w.Code)
	}
	s.rec.Reset()
	s.do("GET", "/api/users/1", "")
	if _, ok := s.rec.Find("looking up user"); ok {
		t.Error("route level still applied after reset")
	}

	if w := s.do("PUT", "/admin/log-levels/default", `{"level":"warn"}`, headerAdminToken, token); w.Code != http.StatusOK {
		t.Fatalf("set default: status %d", w.Code)
	}
	if s.log.Level() != logger.WARN {
		t.Errorf("default level = %v, want WARN", s.log.Level())
	}
	if _, ok := s.rec.Find("default log level changed"); !ok {
		t.Error("default level change was not audited even though NOTICE < WARN")
	}
}

func TestPanicIsLoggedEvenWhenRouteIsOff(t *testing.T) {
	s := newTestServer(t)
	s.levels.Set("GET /api/panic", logger.OFF)
	w := s.do("GET", "/api/panic", "", headerRequestID, "p1")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", w.Code)
	}
	e, ok := s.rec.Find("panic: something went badly wrong")
	if !ok || e.Level != logger.ERROR || !strings.Contains(e.Message, "goroutine") || e.Fields["request_id"] != "p1" {
		t.Fatalf("panic entry = %v (found %v); all: %q", e, ok, s.messages())
	}
	if s.rec.Len() != 1 {
		t.Errorf("OFF route wrote more than the panic: %q", s.messages())
	}
}

func TestUntrustedRequestIDIsReplaced(t *testing.T) {
	s := newTestServer(t)
	for _, bad := range []string{strings.Repeat("x", 65), "abc def", "id\nINFO forged"} {
		w := s.do("GET", "/healthz", "", headerRequestID, bad)
		if got := w.Header().Get(headerRequestID); len(got) != 16 || got == bad {
			t.Errorf("request id %q was not replaced (got %q)", bad, got)
		}
	}
}

func TestCardNumberIsRedactedInJSON(t *testing.T) {
	var buf bytes.Buffer
	opts := logger.Production()
	opts.Output = &buf
	opts.Level = logger.DEBUG
	opts.ReplaceAttr = logger.RedactKeys("card_number")
	router := newRouter(logger.New(opts), NewRouteLevels(), "")

	req := httptest.NewRequest("POST", "/api/orders", strings.NewReader(`{"item":"book","quantity":2,"card_number":"4242424242424242"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	out := buf.String()
	if strings.Contains(out, "4242424242424242") || !strings.Contains(out, `"card_number":"[REDACTED]"`) {
		t.Errorf("card number not redacted:\n%s", out)
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if !json.Valid([]byte(line)) {
			t.Errorf("invalid JSON line: %s", line)
		}
	}
}
